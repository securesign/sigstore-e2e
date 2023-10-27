package keycloak

import (
	"context"
	projectv1 "github.com/openshift/api/project/v1"
	olmV1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
	"sigstore-e2e-test/pkg/api"
	"sigstore-e2e-test/pkg/kubernetes"
	"sigstore-e2e-test/pkg/kubernetes/operator"
	"time"
)

var (
	defaultSubscription = operator.Operator{
		SubscriptionName: "sigstore-keycloak",
		PackageName:      "rhsso-operator",
		Channel:          "stable",
		Source:           "redhat-operators",
		SourceNamespace:  "openshift-marketplace",
		TargetNamespace:  "keycloak-system",
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
	if api.OidcIssuerURL != "" {
		return true, nil
	}
	l, err := kubernetes.K8sClient.CoreV1().Pods(p.Subscription.TargetNamespace).List(p.ctx, metav1.ListOptions{
		LabelSelector: "name=rhsso-operator",
	},
	)
	if err != nil {
		return false, err
	}
	if len(l.Items) == 0 {
		return false, err
	}

	csv, err := operator.GetInstalledClusterServiceVersion(kubernetes.K8sClient, p.ctx, p.Subscription)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return csv.Status.Phase == olmV1alpha1.CSVPhaseSucceeded, err
}

func (p *OperatorInstaller) Setup() error {
	var err error
	p.preinstalled, err = p.IsReady()
	if err != nil {
		return err
	}
	if p.preinstalled {
		logrus.Info("The RH-SSO operator is already running - skipping installation.")
		return nil
	}

	logrus.Info("Installing RH-SSO operator.")
	err = kubernetes.K8sClient.CreateProject(p.ctx, p.Subscription.TargetNamespace)
	if err != nil {
		return err
	}
	if err := operator.InstallFromOperatorHub(kubernetes.K8sClient, p.ctx, p.Subscription); err != nil {
		return err
	}
	return nil
}

func (p *OperatorInstaller) Destroy() error {
	if p.preinstalled {
		logrus.Debug("Skipping preinstalled RH-SSO operator")
		return nil
	} else {
		logrus.Info("Destroying RH-SSO operator")
		if err := operator.DeleteUsingOperatorHub(kubernetes.K8sClient, p.ctx, p.Subscription); err != nil {
			return err
		}
		if err := kubernetes.K8sClient.DeleteProject(p.ctx, p.Subscription.TargetNamespace); err != nil {
			return err
		}

		return wait.PollUntilContextTimeout(p.ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			project := &projectv1.Project{}
			key := controller.ObjectKey{
				Name: p.Subscription.TargetNamespace,
			}
			e := kubernetes.K8sClient.Get(ctx, key, project)
			if e != nil {
				if errors.IsNotFound(e) {
					return true, nil
				}
				return false, e
			}
			return false, nil
		})
	}
}
