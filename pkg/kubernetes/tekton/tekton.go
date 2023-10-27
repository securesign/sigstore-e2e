package tekton

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigstore-e2e-test/pkg/kubernetes"
	"sigstore-e2e-test/pkg/kubernetes/operator"
	"sigstore-e2e-test/pkg/support"
	"time"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	defaultSubscription = operator.Operator{
		SubscriptionName: "sigstore-tekton",
		PackageName:      "openshift-pipelines-operator-rh",
		Channel:          "latest",
		Source:           "redhat-operators",
		SourceNamespace:  "openshift-marketplace",
		TargetNamespace:  "openshift-operators",
	}
)

type OperatorInstaller struct {
	ctx          context.Context
	Subscription operator.Operator
	preinstalled bool
}

func NewOperatorInstaller(ctx context.Context, subscription *operator.Operator) *OperatorInstaller {
	if subscription == nil {
		subscription = &defaultSubscription
	}

	return &OperatorInstaller{
		ctx:          ctx,
		Subscription: *subscription,
	}
}

func (p *OperatorInstaller) IsReady() (bool, error) {
	csvs := &v1alpha1.ClusterServiceVersionList{}
	if err := kubernetes.K8sClient.List(p.ctx, csvs, ctrl.InNamespace(p.Subscription.TargetNamespace), ctrl.HasLabels{"operators.coreos.com/openshift-pipelines-operator-rh.openshift-operators"}); err != nil {
		return false, err
	}
	for _, i := range csvs.Items {
		if i.Status.Phase == "Succeeded" {
			return true, nil
		}
	}

	_, base := kubernetes.K8sClient.Discovery().ServerResourcesForGroupVersion("tekton.dev/v1beta1")
	_, triggers := kubernetes.K8sClient.Discovery().ServerResourcesForGroupVersion("triggers.tekton.dev/v1beta1")

	for _, e := range []error{base, triggers} {
		if e != nil {
			if errors.IsNotFound(e) {
				return false, nil
			}
			return false, e
		}
	}

	return true, nil
}

func (p *OperatorInstaller) Setup() error {
	var err error
	p.preinstalled, err = p.IsReady()
	if err != nil {
		return err
	}
	if p.preinstalled {
		logrus.Info("The openshift-pipelines-operator is already running - skipping installation.")
		return nil
	}
	logrus.Info("Installing openshift-pipelines-operator.")
	err = operator.InstallFromOperatorHub(kubernetes.K8sClient, p.ctx, p.Subscription)
	if err != nil {
		return err
	}

	return support.WaitUntilIsReady(p.ctx, 10*time.Second, 5*time.Minute, p)
}

func (p *OperatorInstaller) Destroy() error {
	if p.preinstalled {
		logrus.Info("Skipping preinstalled openshift-pipelines-operator")
		return nil
	} else {
		logrus.Info("Destroying openshift-pipelines-operator")
		return operator.DeleteUsingOperatorHub(kubernetes.K8sClient, p.ctx, p.Subscription)
	}
}
