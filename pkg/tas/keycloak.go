package tas

import (
	"context"
	v1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"os"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
	"sigstore-e2e-test/pkg/api"
	"sigstore-e2e-test/pkg/kubernetes"
	"sigstore-e2e-test/pkg/kubernetes/keycloak"
	"sigstore-e2e-test/pkg/support"
	"time"
)

const (
	OIDC_REALM = "sigstore"
)

type KeycloakTas struct {
	ctx             context.Context
	keyclock        *keycloak.OperatorInstaller
	tas             *SigstoreOcp
	createResources bool
	resourcesDir    string
	preinstalled    bool
}

func NewKeycloakTas(ctx context.Context, keycloak *keycloak.OperatorInstaller, tas *SigstoreOcp, createResource bool) *KeycloakTas {
	return &KeycloakTas{
		ctx:             ctx,
		keyclock:        keycloak,
		tas:             tas,
		createResources: createResource,
	}
}

func (i *KeycloakTas) IsReady() (bool, error) {
	routeKey := controller.ObjectKey{
		Namespace: i.keyclock.Subscription.TargetNamespace,
		Name:      "keycloak",
	}
	route := &v1.Route{}
	if err := kubernetes.K8sClient.Get(i.ctx, routeKey, route); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return route.Status.Ingress[0].Host != "", nil
}

func (i *KeycloakTas) Setup() error {
	var err error
	i.preinstalled, err = i.IsReady()
	if err != nil {
		return err
	}
	if i.preinstalled {
		logrus.Info("RH-SSO instance is already running - skipping installation.")
		return i.resolveRoutes()
	}

	logrus.Info("Installation of RH-SSO instance")
	err = support.WaitUntilIsReady(i.ctx, 10*time.Second, 5*time.Minute, i.tas, i.keyclock)
	if err != nil {
		return err
	}

	if i.createResources {
		i.resourcesDir = i.tas.RepoDir + "/keycloak/resources/base/"
		entr, err := os.ReadDir(i.resourcesDir)
		if err != nil {
			return err
		}
		for _, e := range entr {
			if e.Name() == "kustomization.yaml" {
				continue
			}
			err := kubernetes.K8sClient.CreateResource(i.ctx, i.keyclock.Subscription.TargetNamespace, i.resourcesDir+e.Name())
			if err != nil {
				return err
			}
		}

		err = support.WaitUntilIsReady(i.ctx, 10*time.Second, 10*time.Minute, i)
		if err != nil {
			return err
		}
	}
	return i.resolveRoutes()
}

func (i *KeycloakTas) Destroy() error {
	if i.preinstalled {
		logrus.Debug("Skipping preinstalled RH-SSO instance")
		return nil
	} else {
		entr, err := os.ReadDir(i.resourcesDir)
		if err != nil {
			return err
		}
		for _, e := range entr {
			if e.Name() == "kustomization.yaml" {
				continue
			}
			if err := kubernetes.K8sClient.DeleteResource(i.ctx, i.keyclock.Subscription.TargetNamespace, i.resourcesDir+e.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (i *KeycloakTas) resolveRoutes() error {
	routeKey := controller.ObjectKey{
		Namespace: i.keyclock.Subscription.TargetNamespace,
		Name:      "keycloak",
	}
	route := &v1.Route{}
	err := kubernetes.K8sClient.Get(i.ctx, routeKey, route)
	if err != nil {
		return err
	}

	api.OidcIssuerURL = "https://" + route.Status.Ingress[0].Host + "/auth/realms/" + OIDC_REALM
	return nil
}
