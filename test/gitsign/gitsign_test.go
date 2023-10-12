package gitsign

import (
	"github.com/google/go-github/v56/github"
	routev1 "github.com/openshift/api/route/v1"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigstore-e2e-test/test/support"
	"testing"
)

func TestSignVerifyCommit(t *testing.T) {
	support.WithNewTestNamespace(func(ns string) {
		// create tekton stuff (see https://github.com/securesign/pipelines-demo/tree/main/01_verify_source_code_pipeline)
		support.TestClient.Create(support.TestContext, createVerifyCommitSignatureTask())
		route := webhookRoute()
		support.TestClient.Create(support.TestContext, route)

		// TODO
		client := github.NewClient(nil).WithAuthToken("")

		client.Repositories.CreateHook(support.TestContext, "", "", &github.Hook{
			URL:    &route.Status.Ingress[0].Host,
			Name:   nil,
			Events: nil,
			Active: nil,
		})

		t.Run("Sign github commit", func(t *testing.T) {
			// configure gitsign, push commit with signature
		})

		t.Run("Verify pipeline run", func(t *testing.T) {
			// verify tekton pipeline run
		})
	})
}

func createVerifyCommitSignatureTask() *v1.Task {
	// TODO
	return &v1.Task{
		Spec: v1.TaskSpec{
			Params:     nil,
			Steps:      nil,
			Workspaces: nil,
			Results:    nil,
		},
	}
}

func webhookRoute() *routev1.Route {
	// TODO
	return &routev1.Route{}
}
