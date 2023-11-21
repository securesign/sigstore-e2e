package kubernetes

import (
	"context"
	"github.com/google/uuid"
	projectv1 "github.com/openshift/api/project/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectPrerequisite struct {
	ctx       context.Context
	Namespace string
	keep      bool
}

func NewTestProject(ctx context.Context, namespace string, keep bool) *ProjectPrerequisite {
	if namespace == "" {
		namespace = "test-" + uuid.New().String()
	}
	return &ProjectPrerequisite{
		ctx:       ctx,
		Namespace: namespace,
		keep:      keep,
	}
}

func (p *ProjectPrerequisite) Setup() error {
	request := &projectv1.ProjectRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: p.Namespace,
		},
	}
	logrus.Info("Creating new project ", p.Namespace)
	return K8sClient.Create(p.ctx, request)
}

func (p *ProjectPrerequisite) Destroy() error {
	if !p.keep {
		logrus.Info("Destroying project ", p.Namespace)
		return K8sClient.Delete(p.ctx, &projectv1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: p.Namespace,
			},
		})
	}
	return nil
}
