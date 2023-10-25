package support

import (
	"context"
	"github.com/google/uuid"
	projectv1 "github.com/openshift/api/project/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"sigstore-e2e-test/pkg/client"
)

var TestNamespace string
var keepNs = false

type ProjectPrerequisite struct {
	ctx context.Context
}

func NewTestProject(ctx context.Context) *ProjectPrerequisite {
	return &ProjectPrerequisite{
		ctx: ctx,
	}
}

func (p ProjectPrerequisite) Install(c client.Client) error {
	TestNamespace = os.Getenv("TEST_NS")
	if TestNamespace == "" {
		TestNamespace = "test-" + uuid.New().String()
	} else {
		keepNs = true
		return nil
	}

	request := &projectv1.ProjectRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: TestNamespace,
		},
	}
	logrus.Info("Creating new project ", TestNamespace)
	return c.Create(p.ctx, request)
}

func (p ProjectPrerequisite) Destroy(c client.Client) error {
	if !keepNs {
		return c.Delete(p.ctx, &projectv1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: TestNamespace,
			},
		})
	}
	return nil
}
