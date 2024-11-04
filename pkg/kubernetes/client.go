package kubernetes

import (
	"context"
	"os"
	"sync"

	consoleCli "github.com/openshift/api/console/v1"
	projectv1 "github.com/openshift/api/project/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var k8sClient Client
var once sync.Once

// Client is an abstraction for a k8s client.
type Client interface {
	controller.Client
	kubernetes.Interface
	GetScheme() *runtime.Scheme
	GetConfig() *rest.Config

	CreateResource(ctx context.Context, ns string, filePath string) error
	DeleteResource(ctx context.Context, ns string, filePath string) error
	CreateProject(ctx context.Context, name string) error
	DeleteProject(ctx context.Context, name string) error
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

func GetClient() Client {
	once.Do(func() {
		var err error
		if k8sClient, err = newClient(); err != nil {
			panic(err)
		}
		_ = projectv1.AddToScheme(k8sClient.GetScheme())
		_ = consoleCli.AddToScheme(k8sClient.GetScheme())
	})
	return k8sClient
}

// NewClient creates a new k8s client that can be used from outside or in the cluster.
func newClient() (Client, error) {
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
	bytes, _ := os.ReadFile(filePath)
	object := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(bytes, object); err != nil {
		return err
	}
	object.SetNamespace(ns)
	return c.Create(ctx, object)
}

func (c *defaultClient) DeleteResource(ctx context.Context, ns string, filePath string) error {
	bytes, _ := os.ReadFile(filePath)
	object := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(bytes, object); err != nil {
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
