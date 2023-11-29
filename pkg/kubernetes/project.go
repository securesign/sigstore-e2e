package kubernetes

import (
	"context"

	"github.com/google/uuid"
	projectv1 "github.com/openshift/api/project/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectPrerequisite struct {
	Namespace string
	keep      bool
}

func NewTestProject(namespace string, keep bool) *ProjectPrerequisite {
	if namespace == "" {
		namespace = "test-" + uuid.New().String()
	}
	return &ProjectPrerequisite{
		Namespace: namespace,
		keep:      keep,
	}
}

func (p *ProjectPrerequisite) Setup(ctx context.Context) error {
	request := &projectv1.ProjectRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: p.Namespace,
		},
	}
	logrus.Info("Creating new project ", p.Namespace)
	return K8sClient.Create(ctx, request)
}

func (p *ProjectPrerequisite) Destroy(ctx context.Context) error {
	if !p.keep {
		logrus.Info("Destroying project ", p.Namespace)
		return K8sClient.Delete(ctx, &projectv1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: p.Namespace,
			},
		})
	}
	return nil
}
