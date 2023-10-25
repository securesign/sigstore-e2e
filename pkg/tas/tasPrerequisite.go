package tas

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	helmClient "github.com/mittwald/go-helm-client"
	configv1 "github.com/openshift/api/config/v1"
	route "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/big"
	"os"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
	"sigstore-e2e-test/pkg/client"
	"strings"
	"time"
)

const (
	FULCIO_NAMESPACE  = "fulcio-system"
	REKOR_NAMESPACE   = "rekor-system"
	TUF_NAMESPACE     = "tuf-system"
	CERT_PASSWORD     = "secretPassword"
	RELEASE_NAME      = "trusted-artifact-signer"
	RELEASE_NAMESPACE = "sigstore"
)

var (
	preinstalled bool
	keycloak     *KeycloakInstaller
	FulcioURL    string
	RekorURL     string
	TufURL       string
)

type tasTestPrerequisite struct {
	ctx     context.Context
	helmCli helmClient.Client
}

func NewTas(ctx context.Context) *tasTestPrerequisite {
	cli, err := helmClient.New(&helmClient.Options{
		Debug:     true,
		Namespace: RELEASE_NAMESPACE,
	})
	if err != nil {
		panic("Can't create helm client")
	}
	return &tasTestPrerequisite{
		ctx:     ctx,
		helmCli: cli,
	}
}

func (p tasTestPrerequisite) isRunning(c client.Client) (bool, error) {
	FulcioURL = os.Getenv("FULCIO_URL")
	RekorURL = os.Getenv("REKOR_URL")
	TufURL = os.Getenv("TUF_URL")

	if FulcioURL != "" && RekorURL != "" && TufURL != "" {
		return true, nil
	}

	releases, err := p.helmCli.ListDeployedReleases()
	if err != nil {
		return false, err
	}
	for _, r := range releases {
		if r.Name == RELEASE_NAME {
			p.resolveRoutes(c)
			return true, nil
		}
	}
	return false, nil
}

func (p tasTestPrerequisite) Install(c client.Client) error {
	// check keycloak installation
	keycloak = NewKeycloakInstaller(p.ctx, true)
	keycloak.Install(c)

	var err error
	preinstalled, err = p.isRunning(c)
	if err != nil {
		return err
	}
	if preinstalled {
		logrus.Info("TAS system is already installed - skipping installation.")
		return nil
	}

	logrus.Info("Installing TAS system.")
	subdomain, err := p.getClusterSubdomain(c)
	if err != nil {
		return err
	}

	c.CreateProject(p.ctx, FULCIO_NAMESPACE)
	c.CreateProject(p.ctx, REKOR_NAMESPACE)

	// create secrets with keys/certs
	public, private, root, err := initCertificates(subdomain, true)
	if err != nil {
		return err
	}
	fulcio := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fulcio-secret-rh",
			Namespace: FULCIO_NAMESPACE,
		},
		StringData: map[string]string{
			"private":  string(private[:]),
			"public":   string(public[:]),
			"cert":     string(root[:]),
			"password": CERT_PASSWORD,
		},
	}
	c.CoreV1().Secrets(FULCIO_NAMESPACE).Create(p.ctx, &fulcio, metav1.CreateOptions{})

	_, private, _, err = initCertificates(subdomain, false)
	rekor := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rekor-private-key",
			Namespace: REKOR_NAMESPACE,
		},
		StringData: map[string]string{
			"private": string(private[:]),
		},
	}
	c.CoreV1().Secrets(REKOR_NAMESPACE).Create(p.ctx, &rekor, metav1.CreateOptions{})

	bytes, _ := os.ReadFile(repoDir + "/examples/values-sigstore-openshift.yaml")
	values := strings.ReplaceAll(string(bytes[:]), "$OPENSHIFT_APPS_SUBDOMAIN", subdomain)
	chartSpec := &helmClient.ChartSpec{
		ReleaseName:     RELEASE_NAME,
		ChartName:       repoDir + "/charts/trusted-artifact-signer",
		Namespace:       RELEASE_NAMESPACE,
		Wait:            true,
		ValuesYaml:      values,
		CreateNamespace: true,
		Timeout:         8 * time.Minute,
	}
	_, err = p.helmCli.InstallOrUpgradeChart(p.ctx, chartSpec, &helmClient.GenericHelmOptions{})
	if err != nil {
		return err
	}
	err = p.resolveRoutes(c)
	return err
}

func (p tasTestPrerequisite) Destroy(c client.Client) error {
	keycloak.Destroy(c)
	if preinstalled {
		logrus.Debug("Skipping preinstalled TAS uninstallation.")
		return nil
	} else {
		logrus.Info("Destroying TAS")
		err := p.helmCli.UninstallRelease(&helmClient.ChartSpec{
			ReleaseName: RELEASE_NAME,
			Namespace:   "sigstore",
			Wait:        true,
			Timeout:     5 * time.Minute,
		})
		c.DeleteProject(p.ctx, FULCIO_NAMESPACE)
		c.DeleteProject(p.ctx, REKOR_NAMESPACE)
		return err
	}
}

func (p tasTestPrerequisite) getClusterSubdomain(c client.Client) (string, error) {
	object := &configv1.DNS{}
	err := c.Get(p.ctx, client2.ObjectKey{
		Name: "cluster",
	}, object)
	return "apps." + object.Spec.BaseDomain, err
}

func initCertificates(domain string, passwordProtected bool) ([]byte, []byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}

	// private
	privateKeyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, nil, err
	}
	var block *pem.Block
	if passwordProtected {
		block, err = x509.EncryptPEMBlock(rand.Reader, "EC PRIVATE KEY", privateKeyBytes, []byte(CERT_PASSWORD), x509.PEMCipher3DES)
		if err != nil {
			return nil, nil, nil, err
		}
	} else {
		block = &pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: privateKeyBytes,
		}
	}
	privateKeyPem := pem.EncodeToMemory(block)

	// public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	publicKeyPem := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyBytes,
		},
	)

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * 10 * time.Hour)

	issuer := pkix.Name{
		CommonName:         domain,
		Country:            []string{"CR"},
		Organization:       []string{"RedHat"},
		Province:           []string{"Czech Republic"},
		Locality:           []string{"Brno"},
		OrganizationalUnit: []string{"QE"},
	}
	//Create certificate templet
	template := x509.Certificate{
		SerialNumber:          big.NewInt(0),
		Subject:               issuer,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		Issuer:                issuer,
	}
	//Create certificate using templet
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, err

	}
	//pem encoding of certificate
	root := pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: derBytes,
		},
	)
	return publicKeyPem, privateKeyPem, root, err
}

func (p tasTestPrerequisite) getRoute(c client.Client, label string, ns string) (*route.Route, error) {
	routeList := &route.RouteList{}
	if err := c.List(p.ctx, routeList, controller.InNamespace(ns), controller.HasLabels{label}); err != nil {
		return nil, err
	}
	if len(routeList.Items) == 0 {
		return nil, fmt.Errorf("route not found")
	}
	return &routeList.Items[0], nil
}

func (p tasTestPrerequisite) resolveRoutes(c client.Client) error {
	fulcioRoute, err := p.getRoute(c, "app.kubernetes.io/name=fulcio", FULCIO_NAMESPACE)
	if err != nil {
		return err
	}
	FulcioURL = "https://" + fulcioRoute.Status.Ingress[0].Host

	rekorRoute, err := p.getRoute(c, "app.kubernetes.io/name=rekor", REKOR_NAMESPACE)
	if err != nil {
		return err
	}
	RekorURL = "https://" + rekorRoute.Status.Ingress[0].Host

	// tuf does not have any meaningful label
	routeList := &route.RouteList{}
	if err := c.List(p.ctx, routeList, controller.InNamespace(TUF_NAMESPACE)); err != nil {
		return err
	}
	if len(routeList.Items) == 0 {
		return fmt.Errorf("can't find TUF route")
	}
	TufURL = "https://" + routeList.Items[0].Status.Ingress[0].Host

	return nil
}
