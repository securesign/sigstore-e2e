package operator

import (
	"context"
	"time"

	"github.com/securesign/sigstore-e2e/pkg/kubernetes"

	olmV1 "github.com/operator-framework/api/pkg/operators/v1"
	olmV1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	controllerruntime "sigs.k8s.io/controller-runtime"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
)

type Operator struct {
	SubscriptionName string
	PackageName      string
	Channel          string
	Source           string
	SourceNamespace  string
	TargetNamespace  string
}

func newSubscription(operator Operator) *olmV1alpha1.Subscription {
	return &olmV1alpha1.Subscription{
		ObjectMeta: controllerruntime.ObjectMeta{
			Name:      operator.SubscriptionName,
			Namespace: operator.TargetNamespace,
		},
		Spec: &olmV1alpha1.SubscriptionSpec{
			CatalogSource:          operator.Source,
			CatalogSourceNamespace: operator.SourceNamespace,
			Package:                operator.PackageName,
			Channel:                operator.Channel,
		},
	}
}

func InstallFromOperatorHub(client kubernetes.Client, ctx context.Context, operator Operator) error {
	if operator.TargetNamespace == "openshift-operators" {
		logrus.Debug("installing as a cluster-wide operator")
	} else {
		ogs := &olmV1.OperatorGroupList{}
		if err := client.List(ctx, ogs, controller.InNamespace(operator.TargetNamespace)); err != nil {
			return err
		}
		if len(ogs.Items) == 0 {
			operatorGroup := olmV1.OperatorGroup{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: operator.SubscriptionName,
					Namespace:    operator.TargetNamespace,
				},
				Spec: olmV1.OperatorGroupSpec{
					TargetNamespaces: []string{operator.TargetNamespace},
				},
			}
			if err := client.Create(ctx, &operatorGroup); err != nil {
				return err
			}
		}
	}

	subscription := newSubscription(operator)
	err := client.Create(ctx, subscription)
	if err != nil {
		return err
	}
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) { //nolint:mnd
		csv, err := GetInstalledClusterServiceVersion(client, ctx, operator)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return csv.Status.Phase == olmV1alpha1.CSVPhaseSucceeded, err
	})
}

func GetInstalledClusterServiceVersion(client kubernetes.Client, ctx context.Context, operator Operator) (*olmV1alpha1.ClusterServiceVersion, error) {
	subscription := &olmV1alpha1.Subscription{}
	if err := client.Get(ctx, controller.ObjectKey{
		Namespace: operator.TargetNamespace,
		Name:      operator.SubscriptionName,
	}, subscription); err != nil {
		return nil, err
	}
	csvName := subscription.Status.InstalledCSV
	if csvName == "" {
		return nil, errors.NewNotFound(olmV1.Resource(olmV1alpha1.ClusterServiceVersionKind), "")
	}

	csv := &olmV1alpha1.ClusterServiceVersion{}
	key := controller.ObjectKey{
		Namespace: operator.TargetNamespace,
		Name:      csvName,
	}
	if err := client.Get(ctx, key, csv); err != nil {
		return nil, err
	}

	return csv, nil
}

func DeleteUsingOperatorHub(client kubernetes.Client, ctx context.Context, operator Operator) error {
	subscription := &olmV1alpha1.Subscription{}
	if err := client.Get(ctx, controller.ObjectKey{
		Namespace: operator.TargetNamespace,
		Name:      operator.SubscriptionName,
	}, subscription); err != nil {
		return err
	}

	csv := &olmV1alpha1.ClusterServiceVersion{}
	if err := client.Get(ctx, controller.ObjectKey{
		Namespace: operator.TargetNamespace,
		Name:      subscription.Status.InstalledCSV,
	}, csv); err != nil {
		return err
	}
	if err := client.Delete(ctx, subscription); err != nil {
		return err
	}
	if err := client.Delete(ctx, csv); err != nil {
		return err
	}
	return nil
}
