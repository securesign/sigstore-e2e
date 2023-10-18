package tas

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/go-git/go-git/v5"
	helmClient "github.com/mittwald/go-helm-client"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/big"
	"os"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
	"sigstore-e2e-test/pkg/client"
	"strings"
	"time"
)

const (
	FULCIO_NAMESPACE     = "fulcio-system"
	rekor_NAMESPACE      = "rekor-system"
	RESOURCES_REPOSITORY = "https://github.com/securesign/sigstore-ocp.git"

	RELEASE_NAME = "trusted-artifact-signer"
)

var (
	preinstalled bool
	FulcioURL    string
	RekorURL     string
	TufURL       string
)

type TestPrerequisite struct {
	ctx     context.Context
	helmCli helmClient.Client
}

func New(ctx context.Context) *TestPrerequisite {
	cli, err := helmClient.New(&helmClient.Options{
		Debug:     true,
		Namespace: "sigstore",
	})
	if err != nil {
		panic("Can't create helm client")
	}
	return &TestPrerequisite{
		ctx:     ctx,
		helmCli: cli,
	}
}

func (p TestPrerequisite) isRunning() (bool, error) {
	FulcioURL = os.Getenv("FULCIO_URL")
	RekorURL = os.Getenv("REKOR_URL")
	TufURL = os.Getenv("TUF_URL")
	return false, nil
}

func (p TestPrerequisite) Install(c client.Client) error {
	var err error
	preinstalled, err = p.isRunning()
	if err != nil {
		return err
	}
	if preinstalled {
		logrus.Info("Using preinstalled TAS system")
		return nil
	}

	dir, err := os.MkdirTemp("", "sigstore-ocp")
	if err != nil {
		return err
	}
	_, err = git.PlainClone(dir, false, &git.CloneOptions{
		URL:      RESOURCES_REPOSITORY,
		Progress: os.Stdout,
	})
	if err != nil {
		return err
	}
	subdomain, err := p.getClusterSubdomain(c)
	if err != nil {
		return err
	}

	private, public, root, err := initFulcioCertificates(subdomain)
	fulcio := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fulcio-secret-rh",
			Namespace: FULCIO_NAMESPACE,
		},
		StringData: map[string]string{
			"private": string(private[:]),
			"public":  string(public[:]),
			"cert":    string(root[:]),
		},
	}

	c.CoreV1().Secrets(FULCIO_NAMESPACE).Create(p.ctx, &fulcio, metav1.CreateOptions{})

	private, _, _, err = initFulcioCertificates(subdomain)
	rekor := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rekor-private-key",
			Namespace: rekor_NAMESPACE,
		},
		StringData: map[string]string{
			"private": string(private[:]),
		},
	}
	c.CoreV1().Secrets(rekor_NAMESPACE).Create(p.ctx, &rekor, metav1.CreateOptions{})

	byte, _ := os.ReadFile(dir + "/examples/values-sigstore-openshift.yaml")

	values := strings.ReplaceAll(string(byte[:]), "$OPENSHIFT_APPS_SUBDOMAIN", subdomain)
	chartSpec := &helmClient.ChartSpec{
		ReleaseName:     RELEASE_NAME,
		ChartName:       dir + "/charts/trusted-artifact-signer",
		Namespace:       "sigstore",
		Wait:            true,
		ValuesYaml:      values,
		CreateNamespace: true,
		Timeout:         5 * time.Minute,
	}
	_, err = p.helmCli.InstallOrUpgradeChart(p.ctx, chartSpec, &helmClient.GenericHelmOptions{})
	return err
}

func (p TestPrerequisite) Destroy(c client.Client) error {
	if preinstalled {
		logrus.Info("Skipping preinstalled openshift-pipelines-operator")
		return nil
	} else {
		return p.helmCli.UninstallRelease(&helmClient.ChartSpec{
			ReleaseName: RELEASE_NAME,
			Namespace:   "sigstore",
			Wait:        true,
		})
	}
}

func (p TestPrerequisite) getClusterSubdomain(c client.Client) (string, error) {
	object := &configv1.DNS{}
	err := c.Get(p.ctx, client2.ObjectKey{
		Name: "cluster",
	}, object)
	return object.Spec.BaseDomain, err
}

func initFulcioCertificates(domain string) ([]byte, []byte, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, err
	}
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(key)
	block, err := x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", privateKeyBytes, []byte("mypassword"), x509.PEMCipher3DES)
	if err != nil {
		panic(err)
	}
	// PEM encoding of private key
	privateKeyPem := pem.EncodeToMemory(block)

	publicKeyBytes := x509.MarshalPKCS1PublicKey(&key.PublicKey)
	publicKeyPem := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: publicKeyBytes,
		},
	)

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * 10 * time.Hour)

	//Create certificate templet
	template := x509.Certificate{
		SerialNumber:          big.NewInt(0),
		Subject:               pkix.Name{CommonName: domain},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
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
