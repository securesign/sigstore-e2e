package tekton

import (
	"context"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
	"sigstore-e2e-test/pkg/client"
	"time"
)

const (
	SUBSCRIPTION_NAME = "sigstore-tekton"
	PACKAGE_NAME      = "openshift-pipelines-operator-rh"
	CHANNEL           = "latest"
	SOURCE            = "redhat-operators"
	SOURCE_NAMESPACE  = "openshift-marketplace"
	TARGET_NAMESPACE  = "openshift-operators"
)

var preinstalled bool

type tektonPrerequisite struct {
	ctx context.Context
}

func NewTektonInstaller(ctx context.Context) *tektonPrerequisite {
	return &tektonPrerequisite{
		ctx: ctx,
	}
}

func (p tektonPrerequisite) IsReady(c client.Client) bool {
	csvs := &v1alpha1.ClusterServiceVersionList{}
	if err := c.List(p.ctx, csvs, ctrl.InNamespace(TARGET_NAMESPACE), ctrl.HasLabels{"operators.coreos.com/openshift-pipelines-operator-rh.openshift-operators"}); err != nil {
		return false
	}
	for _, i := range csvs.Items {
		if i.Status.Phase != "Succeeded" {
			return false
		}
	}
	// check tekton resources
	if _, base := c.Discovery().ServerResourcesForGroupVersion("tekton.dev/v1beta1"); errors.IsNotFound(base) {
		return false
	}

	if _, triggers := c.Discovery().ServerResourcesForGroupVersion("triggers.tekton.dev/v1beta1"); errors.IsNotFound(triggers) {
		return false
	}
	return true
}

func (p tektonPrerequisite) Install(c client.Client) error {
	preinstalled = p.IsReady(c)
	if preinstalled {
		logrus.Info("The openshift-pipelines-operator is already running - skipping installation.")
		return nil
	}
	logrus.Info("Installing openshift-pipelines-operator.")
	err := c.InstallFromOperatorHub(p.ctx, SUBSCRIPTION_NAME, TARGET_NAMESPACE, PACKAGE_NAME, CHANNEL, SOURCE, SOURCE_NAMESPACE)
	if err != nil {
		return err
	}

	return wait.PollUntilContextTimeout(p.ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		return p.IsReady(c), nil
	})
}

func (p tektonPrerequisite) Destroy(c client.Client) error {
	if preinstalled {
		logrus.Info("Skipping preinstalled openshift-pipelines-operator")
		return nil
	} else {
		logrus.Info("Destroying openshift-pipelines-operator")
		return c.DeleteUsingOperatorHub(p.ctx, SUBSCRIPTION_NAME, TARGET_NAMESPACE)
	}
}
