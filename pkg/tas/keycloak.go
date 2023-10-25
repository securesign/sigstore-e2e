package tas

import (
	"context"
	v1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
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

type keycloakInstaller struct {
	ctx             context.Context
	createResources bool
}

func NewKeycloakInstaller(ctx context.Context, createResources bool) *keycloakInstaller {
	return &keycloakInstaller{
		ctx:             ctx,
		createResources: createResources,
	}
}

func (p keycloakInstaller) isRunning(c client.Client) (bool, error) {
	OidcIssuerURL = os.Getenv("OIDC_ISSUER_URL")
	if OidcIssuerURL != "" {
		return true, nil
	}
	l, err := c.CoreV1().Pods(TARGET_NAMESPACE).List(p.ctx, metav1.ListOptions{
		LabelSelector: "name=rhsso-operator",
	},
	)
	if err != nil {
		return false, err
	}
	if len(l.Items) == 0 {
		return false, err
	}
	err = p.resolveIssuerUrl(c)
	return true, err
}

func (p keycloakInstaller) Install(c client.Client) error {
	var err error
	keycloakPreinstalled, err = p.isRunning(c)
	if err != nil {
		return err
	}
	if keycloakPreinstalled {
		logrus.Info("The RH-SSO-operator is already running - skipping installation.")
		return nil
	}

	logrus.Info("Installing RH-SSO system.")
	c.CreateProject(p.ctx, TARGET_NAMESPACE)
	if err := c.InstallFromOperatorHub(p.ctx, SUBSCRIPTION_NAME, TARGET_NAMESPACE, PACKAGE_NAME, CHANNEL, SOURCE, SOURCE_NAMESPACE); err != nil {
		return err
	}
	if p.createResources {
		resourcesDir = repoDir + "/keycloak/resources/base"
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

		routeKey := controller.ObjectKey{
			Namespace: TARGET_NAMESPACE,
			Name:      "keycloak",
		}
		// wait for keycloak route
		route := &v1.Route{}
		wait.PollUntilContextTimeout(p.ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			if err := c.Get(ctx, routeKey, route); err != nil {
				if errors.IsNotFound(err) {
					return false, nil
				} else {
					return false, err
				}
			}
			return route.Status.Ingress[0].Host != "", nil
		})
		p.resolveIssuerUrl(c)
	}
	return nil
}

func (p keycloakInstaller) Destroy(c client.Client) error {
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
		c.DeleteProject(p.ctx, TARGET_NAMESPACE)
		return err
	}
}

func (p keycloakInstaller) resolveIssuerUrl(c client.Client) error {
	routeKey := controller.ObjectKey{
		Namespace: TARGET_NAMESPACE,
		Name:      "keycloak",
	}
	route := &v1.Route{}
	err := c.Get(p.ctx, routeKey, route)
	OidcIssuerURL = "https://" + route.Status.Ingress[0].Host + "/auth/realms/" + OIDC_REALM
	return err

}
