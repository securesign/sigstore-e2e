package tas

import (
	"context"
	"fmt"
	helmClient "github.com/mittwald/go-helm-client"
	configv1 "github.com/openshift/api/config/v1"
	route "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
	"sigstore-e2e-test/pkg/api"
	"sigstore-e2e-test/pkg/kubernetes"
	"sigstore-e2e-test/pkg/support"
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
)

type HelmInstaller struct {
	ctx       context.Context
	component *SigstoreOcp
	helmCli   helmClient.Client
}

func NewHelmInstaller(ctx context.Context, component *SigstoreOcp) *HelmInstaller {
	cli, err := helmClient.New(&helmClient.Options{
		Debug:     true,
		Namespace: RELEASE_NAMESPACE,
	})
	if err != nil {
		panic("Can't create helm client")
	}
	return &HelmInstaller{
		ctx:       ctx,
		component: component,
		helmCli:   cli,
	}
}

func (p *HelmInstaller) IsReady() (bool, error) {
	if api.FulcioURL != "" && api.RekorURL != "" && api.TufURL != "" {
		return true, nil
	}
	if err := p.resolveRoutes(); err != nil {
		return false, nil
	}
	return true, nil
}

func (p *HelmInstaller) Setup() error {
	var err error

	err = support.WaitUntilIsReady(p.ctx, 10*time.Second, 2*time.Minute, p.component)
	if err != nil {
		return err
	}

	preinstalled, err = p.IsReady()
	if err != nil {
		return err
	}
	if preinstalled {
		logrus.Info("TAS system is already installed - skipping installation.")
		return nil
	}

	logrus.Info("Installing TAS system.")
	subdomain, err := p.getClusterSubdomain()
	if err != nil {
		return err
	}

	kubernetes.K8sClient.CreateProject(p.ctx, FULCIO_NAMESPACE)
	kubernetes.K8sClient.CreateProject(p.ctx, REKOR_NAMESPACE)

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
	kubernetes.K8sClient.CoreV1().Secrets(FULCIO_NAMESPACE).Create(p.ctx, &fulcio, metav1.CreateOptions{})

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
	kubernetes.K8sClient.CoreV1().Secrets(REKOR_NAMESPACE).Create(p.ctx, &rekor, metav1.CreateOptions{})

	bytes, _ := os.ReadFile(p.component.RepoDir + "/examples/values-sigstore-openshift.yaml")
	values := strings.ReplaceAll(string(bytes[:]), "$OPENSHIFT_APPS_SUBDOMAIN", subdomain)
	chartSpec := &helmClient.ChartSpec{
		ReleaseName:     RELEASE_NAME,
		ChartName:       p.component.RepoDir + "/charts/trusted-artifact-signer",
		Namespace:       RELEASE_NAMESPACE,
		Wait:            true,
		ValuesYaml:      values,
		CreateNamespace: true,
		Timeout:         10 * time.Minute,
	}
	_, err = p.helmCli.InstallOrUpgradeChart(p.ctx, chartSpec, &helmClient.GenericHelmOptions{})
	if err != nil {
		return err
	}
	err = p.resolveRoutes()
	return err
}

func (p *HelmInstaller) Destroy() error {
	if preinstalled {
		logrus.Debug("Skipping preinstalled TAS uninstallation.")
		return nil
	} else {
		logrus.Info("Destroying TAS")
		err := p.helmCli.UninstallRelease(&helmClient.ChartSpec{
			ReleaseName: RELEASE_NAME,
			Namespace:   "sigstore",
			Wait:        true,
			Timeout:     10 * time.Minute,
		})
		kubernetes.K8sClient.DeleteProject(p.ctx, FULCIO_NAMESPACE)
		kubernetes.K8sClient.DeleteProject(p.ctx, REKOR_NAMESPACE)
		return err
	}
}

func (p *HelmInstaller) getClusterSubdomain() (string, error) {
	object := &configv1.DNS{}
	err := kubernetes.K8sClient.Get(p.ctx, client2.ObjectKey{
		Name: "cluster",
	}, object)
	return "apps." + object.Spec.BaseDomain, err
}

func (p *HelmInstaller) getRoute(label string, ns string) (*route.Route, error) {
	routeList := &route.RouteList{}
	if err := kubernetes.K8sClient.List(p.ctx, routeList, controller.InNamespace(ns), controller.HasLabels{label}); err != nil {
		return nil, err
	}
	if len(routeList.Items) == 0 {
		return nil, fmt.Errorf("route not found")
	}
	return &routeList.Items[0], nil
}

func (p *HelmInstaller) resolveRoutes() error {
	fulcioRoute, err := p.getRoute("app.kubernetes.io/name=fulcio", FULCIO_NAMESPACE)
	if err != nil {
		return err
	}
	api.FulcioURL = "https://" + fulcioRoute.Status.Ingress[0].Host

	rekorRoute, err := p.getRoute("app.kubernetes.io/name=rekor", REKOR_NAMESPACE)
	if err != nil {
		return err
	}
	api.RekorURL = "https://" + rekorRoute.Status.Ingress[0].Host

	// tuf does not have any meaningful label
	routeList := &route.RouteList{}
	if err := kubernetes.K8sClient.List(p.ctx, routeList, controller.InNamespace(TUF_NAMESPACE)); err != nil {
		return err
	}
	if len(routeList.Items) == 0 {
		return fmt.Errorf("can't find TUF route")
	}
	api.TufURL = "https://" + routeList.Items[0].Status.Ingress[0].Host

	return nil
}
