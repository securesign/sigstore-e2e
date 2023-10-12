package tekton

import (
	"context"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
	"sigstore-e2e-test/pkg/client"
)

const SUBSCRIPTION_NAME = "sigstore-tekton"
const PACKAGE_NAME = "openshift-pipelines-operator-rh"
const CHANNEL = "latest"
const SOURCE = "redhat-operators"
const SOURCE_NAMESPACE = "openshift-marketplace"

const TARGET_NAMESPACE = "openshift-operators"

var preinstalled bool

type TektonTestPrerequisite struct {
	ctx context.Context
}

func New(ctx context.Context) *TektonTestPrerequisite {
	return &TektonTestPrerequisite{
		ctx: ctx,
	}
}

func (p TektonTestPrerequisite) isRunning(c client.Client) (bool, error) {
	csvs := &v1alpha1.ClusterServiceVersionList{}
	if err := c.List(p.ctx, csvs, ctrl.InNamespace(TARGET_NAMESPACE), ctrl.HasLabels{"operators.coreos.com/openshift-pipelines-operator-rh.openshift-operators"}); err != nil {
		return false, err
	}
	for _, i := range csvs.Items {
		if i.Status.Phase == "Succeeded" {
			return true, nil
		}
	}
	return false, nil
}

func (p TektonTestPrerequisite) Install(c client.Client) error {
	var err error
	preinstalled, err = p.isRunning(c)
	if err != nil {
		return err
	}
	if preinstalled {
		logrus.Info("The openshift-pipelines-operator is already running")
		return nil
	}
	c.InstallFromOperatorHub(p.ctx, SUBSCRIPTION_NAME, TARGET_NAMESPACE, PACKAGE_NAME, CHANNEL, SOURCE, SOURCE_NAMESPACE)
	return nil
}

func (p TektonTestPrerequisite) Destroy(c client.Client) error {
	if preinstalled {
		logrus.Info("Skipping preinstalled openshift-pipelines-operator")
		return nil
	} else {
		return c.DeleteUsingOperatorHub(p.ctx, SUBSCRIPTION_NAME, TARGET_NAMESPACE)
	}
}
