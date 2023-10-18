package testSupport

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	configv1 "github.com/openshift/api/config/v1"
	projectv1 "github.com/openshift/api/project/v1"
	routev1 "github.com/openshift/api/route/v1"
	olmV1 "github.com/operator-framework/api/pkg/operators/v1"
	olmV1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	tektonTriggers "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"sigstore-e2e-test/pkg/client"
	"sigstore-e2e-test/pkg/support"
	"sync"
)

var TestClient client.Client
var TestContext context.Context

var prerequistities []support.TestPrerequisite

func init() {
	var err error

	TestContext = context.TODO()
	if TestClient, err = client.NewClient(); err != nil {
		panic(err)
	}

	olmV1alpha1.AddToScheme(TestClient.GetScheme())
	olmV1.AddToScheme(TestClient.GetScheme())
	projectv1.AddToScheme(TestClient.GetScheme())
	routev1.AddToScheme(TestClient.GetScheme())
	tektonTriggers.AddToScheme(TestClient.GetScheme())
	configv1.AddToScheme(TestClient.GetScheme())

}

func InstallPrerequisites(prerequisite ...support.TestPrerequisite) error {
	prerequistities = prerequisite
	wg := new(sync.WaitGroup)
	wg.Add(len(prerequisite))
	var errors []error
	for _, i := range prerequisite {
		go func(p support.TestPrerequisite) {
			err := p.Install(TestClient)
			if err != nil {
				errors = append(errors, err)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	if len(errors) != 0 {
		return fmt.Errorf("can't install all prerequisities %s", errors)
	}
	return nil
}

func DestroyPrerequisities() error {
	wg := new(sync.WaitGroup)
	wg.Add(len(prerequistities))
	var errors []error
	for _, i := range prerequistities {
		go func() {
			err := i.Destroy(TestClient)
			if err != nil {
				errors = append(errors, err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if len(errors) != 0 {
		return fmt.Errorf("can't destroy all prerequisities %s", errors)
	}
	return nil
}

func WithNewTestNamespace(doRun func(string)) {
	keepNs := true
	name := os.Getenv("TEST_NS")
	if name == "" {
		keepNs = false
		name = "test-" + uuid.New().String()
	}

	request := &projectv1.ProjectRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	logrus.Info("Creating new project ", name)
	TestClient.Create(TestContext, request)
	defer func() {
		if !keepNs {
			TestClient.Delete(TestContext, &projectv1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			})
		}
	}()

	doRun(name)
}
