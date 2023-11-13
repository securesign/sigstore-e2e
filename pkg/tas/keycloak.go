package tas

import (
	"context"
	v1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"os"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
	"sigstore-e2e-test/pkg/client"
	"time"
)

const (
	SUBSCRIPTION_NAME = "sigstore-keycloak"
	PACKAGE_NAME      = "rhsso-operator"
	CHANNEL           = "stable"
	SOURCE            = "redhat-operators"
	SOURCE_NAMESPACE  = "openshift-marketplace"
	TARGET_NAMESPACE  = "keycloak-system"
	OIDC_REALM        = "sigstore"
)

var (
	keycloakPreinstalled bool
	resourcesDir         string
	OidcIssuerURL        string
)

type KeycloakInstaller struct {
	ctx             context.Context
	createResources bool
}

func NewKeycloakInstaller(ctx context.Context, createResources bool) *KeycloakInstaller {
	return &KeycloakInstaller{
		ctx:             ctx,
		createResources: createResources,
	}
}

func (p KeycloakInstaller) IsReady(c client.Client) bool {
	OidcIssuerURL = os.Getenv("OIDC_ISSUER_URL")
	if OidcIssuerURL != "" {
		return true
	}
	l, _ := c.CoreV1().Pods(TARGET_NAMESPACE).List(p.ctx, metav1.ListOptions{
		LabelSelector: "name=rhsso-operator",
	})
	if len(l.Items) == 0 {
		return false
	}
	if !p.createResources {
		return true
	}
	return p.resolveIssuerUrl(c) == nil
}

func (p KeycloakInstaller) Install(c client.Client) error {
	keycloakPreinstalled = p.IsReady(c)

	if keycloakPreinstalled {
		logrus.Info("The RH-SSO-operator is already running - skipping installation.")
		return nil
	}

	logrus.Info("Installing RH-SSO system.")
	err := c.CreateProject(p.ctx, TARGET_NAMESPACE)
	if err != nil {
		return err
	}
	if err := c.InstallFromOperatorHub(p.ctx, SUBSCRIPTION_NAME, TARGET_NAMESPACE, PACKAGE_NAME, CHANNEL, SOURCE, SOURCE_NAMESPACE); err != nil {
		return err
	}
	if p.createResources {
		resourcesDir = repoDir + "/keycloak/resources/base/"
		entr, err := os.ReadDir(resourcesDir)
		if err != nil {
			return err
		}
		for _, e := range entr {
			if e.Name() == "kustomization.yaml" {
				continue
			}
			err := c.CreateResource(p.ctx, TARGET_NAMESPACE, resourcesDir+e.Name())
			if err != nil {
				return err
			}
		}
	}
	return wait.PollUntilContextTimeout(p.ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		return p.IsReady(c), nil
	})
}

func (p KeycloakInstaller) Destroy(c client.Client) error {
	if keycloakPreinstalled {
		logrus.Debug("Skipping preinstalled RH-SSO operator")
		return nil
	} else {
		logrus.Info("Destroying RH-SSO")
		entr, err := os.ReadDir(resourcesDir)
		if err != nil {
			return err
		}
		for _, e := range entr {
			if e.Name() == "kustomization.yaml" {
				continue
			}
			if err := c.DeleteResource(p.ctx, TARGET_NAMESPACE, resourcesDir+e.Name()); err != nil {
				return err
			}
		}

		err = c.DeleteUsingOperatorHub(p.ctx, SUBSCRIPTION_NAME, TARGET_NAMESPACE)
		_ = c.DeleteProject(p.ctx, TARGET_NAMESPACE)
		return err
	}
}

func (p KeycloakInstaller) resolveIssuerUrl(c client.Client) error {
	routeKey := controller.ObjectKey{
		Namespace: TARGET_NAMESPACE,
		Name:      "keycloak",
	}
	route := &v1.Route{}
	err := c.Get(p.ctx, routeKey, route)
	if err != nil {
		return err
	}
	OidcIssuerURL = "https://" + route.Status.Ingress[0].Host + "/auth/realms/" + OIDC_REALM
	return nil

}
