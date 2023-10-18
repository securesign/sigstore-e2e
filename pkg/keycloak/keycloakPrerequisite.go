package keycloak

import (
	"context"
	"github.com/go-git/go-git/v5"
	projectv1 "github.com/openshift/api/project/v1"
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
	SUBSCRIPTION_NAME    = "sigstore-keycloak"
	PACKAGE_NAME         = "rhsso-operator"
	CHANNEL              = "stable"
	SOURCE               = "redhat-operators"
	SOURCE_NAMESPACE     = "openshift-marketplace"
	TARGET_NAMESPACE     = "keycloak-system"
	RESOURCES_REPOSITORY = "https://github.com/securesign/sigstore-ocp.git"
	OIDC_REALM           = "sigstore"
)

var (
	preinstalled  bool
	resourcesDir  string
	OidcIssuerURL string
)

type TestPrerequisite struct {
	ctx             context.Context
	createResources bool
}

func New(ctx context.Context, createResources bool) *TestPrerequisite {
	return &TestPrerequisite{
		ctx:             ctx,
		createResources: createResources,
	}
}

func (p TestPrerequisite) isRunning(c client.Client) (bool, error) {
	l, err := c.CoreV1().Pods(TARGET_NAMESPACE).List(p.ctx, metav1.ListOptions{
		LabelSelector: "name=rhsso-operator",
	},
	)
	if err != nil {
		return false, err
	}
	OidcIssuerURL = os.Getenv("OIDC_ISSUER_URL")
	return len(l.Items) > 0 && OidcIssuerURL != "", nil
}

func (p TestPrerequisite) Install(c client.Client) error {
	var err error
	preinstalled, err = p.isRunning(c)
	if err != nil {
		return err
	}
	if preinstalled {
		logrus.Info("The RH-SSO-operator is already running")
		return nil
	}

	request := &projectv1.ProjectRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: TARGET_NAMESPACE,
		},
	}
	logrus.Info("Creating new project ", TARGET_NAMESPACE)
	c.Create(p.ctx, request)
	if err := c.InstallFromOperatorHub(p.ctx, SUBSCRIPTION_NAME, TARGET_NAMESPACE, PACKAGE_NAME, CHANNEL, SOURCE, SOURCE_NAMESPACE); err != nil {
		return err
	}
	if p.createResources {
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

		resourcesDir = dir + "/keycloak/resources/"
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
		OidcIssuerURL = route.Status.Ingress[0].Host + "/auth/realms/" + OIDC_REALM
	}
	return nil
}

func (p TestPrerequisite) Destroy(c client.Client) error {
	if preinstalled {
		logrus.Info("Skipping preinstalled RH-SSO operator")
		return nil
	} else {
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
		prj := &projectv1.Project{
			TypeMeta: metav1.TypeMeta{
				APIVersion: projectv1.GroupVersion.String(),
				Kind:       "Project",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: TARGET_NAMESPACE,
			},
		}
		if err := c.Delete(p.ctx, prj); err != nil {
			logrus.Warning("Warning: cannot delete test project ", prj.Name)
		}
		return err
	}
}
