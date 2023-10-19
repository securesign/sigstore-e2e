package client

import (
	"context"
	projectv1 "github.com/openshift/api/project/v1"
	olmV1 "github.com/operator-framework/api/pkg/operators/v1"
	olmV1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"os"
	controllerruntime "sigs.k8s.io/controller-runtime"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"time"
)

// Client is an abstraction for a k8s client
type Client interface {
	controller.Client
	kubernetes.Interface
	GetScheme() *runtime.Scheme
	GetConfig() *rest.Config

	CreateResource(ctx context.Context, ns string, filePath string) error
	DeleteResource(ctx context.Context, ns string, filePath string) error
	CreateProject(ctx context.Context, name string) error
	DeleteProject(ctx context.Context, name string) error
	InstallFromOperatorHub(context context.Context, name string, targetNamespace string, packageName string, channel string, source string, sourceNamespace string) error
	DeleteUsingOperatorHub(ctx context.Context, name string, targetNamespace string) error
}

type defaultClient struct {
	controller.Client
	kubernetes.Interface
	scheme *runtime.Scheme
	config *rest.Config
}

func (c *defaultClient) GetScheme() *runtime.Scheme {
	return c.scheme
}

func (c *defaultClient) GetConfig() *rest.Config {
	return c.config
}

// NewClient creates a new k8s client that can be used from outside or in the cluster
func NewClient() (Client, error) {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	scheme := clientscheme.Scheme

	var clientset kubernetes.Interface
	if clientset, err = kubernetes.NewForConfig(cfg); err != nil {
		return nil, err
	}

	// Create a new client to avoid using cache (enabled by default on operator-sdk client)
	clientOptions := controller.Options{
		Scheme: scheme,
	}
	dynClient, err := controller.New(cfg, clientOptions)
	if err != nil {
		return nil, err
	}

	return &defaultClient{
		Client:    dynClient,
		Interface: clientset,
		scheme:    clientOptions.Scheme,
		config:    cfg,
	}, nil
}

func (c *defaultClient) CreateResource(ctx context.Context, ns string, filePath string) error {
	byte, _ := os.ReadFile(filePath)
	object := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(byte, object); err != nil {
		return err
	}
	object.SetNamespace(ns)
	return c.Create(ctx, object)
}

func (c *defaultClient) DeleteResource(ctx context.Context, ns string, filePath string) error {
	byte, _ := os.ReadFile(filePath)
	object := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(byte, object); err != nil {
		return err
	}
	object.SetNamespace(ns)
	return c.Delete(ctx, object)
}

func (c *defaultClient) CreateProject(ctx context.Context, name string) error {
	request := &projectv1.ProjectRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	logrus.Debug("Creating new project ", name)
	return c.Create(ctx, request)
}
func (c *defaultClient) DeleteProject(ctx context.Context, name string) error {
	return c.Delete(ctx, &projectv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	})
}

func (c *defaultClient) InstallFromOperatorHub(ctx context.Context, name string, targetNamespace string, packageName string, channel string, source string, sourceNamespace string) error {
	if targetNamespace == "openshift-operators" {
		logrus.Debug("installing as a cluster-wide operator")
	} else {
		ogs := &olmV1.OperatorGroupList{}
		if err := c.List(ctx, ogs, ctrl.InNamespace(targetNamespace)); err != nil {
			return err
		}
		if len(ogs.Items) == 0 {
			operatorGroup := olmV1.OperatorGroup{
				ObjectMeta: v1.ObjectMeta{
					GenerateName: name,
					Namespace:    targetNamespace,
				},
				Spec: olmV1.OperatorGroupSpec{
					TargetNamespaces: []string{targetNamespace},
				},
			}
			if err := c.Create(ctx, &operatorGroup); err != nil {
				return err
			}
		}
	}

	subscription := &olmV1alpha1.Subscription{
		ObjectMeta: controllerruntime.ObjectMeta{
			Name:      name,
			Namespace: targetNamespace,
		},
		Spec: &olmV1alpha1.SubscriptionSpec{
			CatalogSource:          source,
			CatalogSourceNamespace: sourceNamespace,
			Package:                packageName,
			Channel:                channel,
		},
	}

	err := c.Create(ctx, subscription)
	if err != nil {
		return err
	}
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		subscriptionKey := controller.ObjectKey{
			Namespace: targetNamespace,
			Name:      name,
		}
		if err := c.Get(ctx, subscriptionKey, subscription); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			} else {
				return false, err
			}
		}
		csvName := subscription.Status.InstalledCSV
		if csvName == "" {
			return false, nil
		}
		key := controller.ObjectKey{
			Namespace: targetNamespace,
			Name:      csvName,
		}

		csv := &olmV1alpha1.ClusterServiceVersion{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterServiceVersion",
				APIVersion: olmV1alpha1.SchemeGroupVersion.String(),
			},
		}
		if err := c.Get(ctx, key, csv); err != nil {
			return false, err
		}
		return csv.Status.Phase == olmV1alpha1.CSVPhaseSucceeded, nil
	})
}

func (c *defaultClient) DeleteUsingOperatorHub(ctx context.Context, name string, targetNamespace string) error {
	subscription := &olmV1alpha1.Subscription{}
	if err := c.Get(ctx, controller.ObjectKey{
		Namespace: targetNamespace,
		Name:      name,
	}, subscription); err != nil {
		return err
	}

	csv := &olmV1alpha1.ClusterServiceVersion{}
	if err := c.Get(ctx, controller.ObjectKey{
		Namespace: targetNamespace,
		Name:      subscription.Status.InstalledCSV,
	}, csv); err != nil {
		return err
	}
	if err := c.Delete(ctx, subscription); err != nil {
		return err
	}
	if err := c.Delete(ctx, csv); err != nil {
		return err
	}
	return nil
}
