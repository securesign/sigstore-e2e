package gitsign

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	gitAuth "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v56/github"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/route/v1"
	"github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path/filepath"
	"runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigstore-e2e-test/pkg/support"
	"sigstore-e2e-test/pkg/tas"
	"sigstore-e2e-test/pkg/tas/gitsign"
	"sigstore-e2e-test/test/testSupport"
	"testing"
	"time"
)

const GITHUB_TOKEN = "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"
const GITHUB_OWNER = "bouskaJ"
const GITHUB_REPO = "gitsign-demo-test"

func TestSignVerifyCommit(t *testing.T) {
	testSupport.WithNewTestNamespace(func(ns string) {
		RegisterTestingT(t)

		// use file definition for large tekton resources
		_, b, _, _ := runtime.Caller(0)
		path := filepath.Dir(b) + "/resources"
		testSupport.TestClient.CreateResource(testSupport.TestContext, ns, path+"/verify-commit-signature-task.yaml")
		testSupport.TestClient.CreateResource(testSupport.TestContext, ns, path+"/verify-source-code-pipeline.yaml")
		testSupport.TestClient.CreateResource(testSupport.TestContext, ns, path+"/verify-source-code-triggertemplate.yaml")
		testSupport.TestClient.CreateResource(testSupport.TestContext, ns, path+"/verify-source-el.yaml")
		testSupport.TestClient.CreateResource(testSupport.TestContext, ns, path+"/verify-source-el-route.yaml")
		testSupport.TestClient.CreateResource(testSupport.TestContext, ns, path+"/webhook-secret-securesign-pipelines-demo.yaml")
		testSupport.TestClient.CreateResource(testSupport.TestContext, ns, path+"/github-push-triggerbinding.yaml")
		Expect(testSupport.TestClient.Create(testSupport.TestContext, createTriggerBindingResource(ns))).To(Succeed())

		route := &v1.Route{}
		Eventually(func() v1.Route {
			testSupport.TestClient.Get(testSupport.TestContext, client.ObjectKey{
				Namespace: ns,
				Name:      "el-verify-source",
			}, route)
			return *route
		}, time.Minute).Should(Not(BeNil()))
		Eventually(route.Status.Ingress[0].Host).Should(Not(BeNil()))

		// register webhook
		// TODO: migrate somewhere else
		hookConfig := make(map[string]interface{})
		hookConfig["url"] = "https://" + route.Status.Ingress[0].Host
		hookConfig["content_type"] = "json"
		hookConfig["secret"] = "secretToken"
		hookName := "test" + uuid.New().String()
		client := github.NewClient(nil).WithAuthToken(GITHUB_TOKEN)
		hook, response, err := client.Repositories.CreateHook(testSupport.TestContext, GITHUB_OWNER, GITHUB_REPO, &github.Hook{
			Name:   &hookName,
			Config: hookConfig,
			Events: []string{"push"},
		})
		Expect(err).To(BeNil())
		Expect(response.Status).To(Equal("201 Created"))
		defer client.Repositories.DeleteHook(testSupport.TestContext, GITHUB_OWNER, GITHUB_REPO, *hook.ID)

		t.Run("Sign github commit", func(t *testing.T) {
			dir, repo, err := support.GitClone(fmt.Sprintf("https://github.com/%s/%s.git", GITHUB_OWNER, GITHUB_REPO))
			Expect(err).To(BeNil())

			config, err := repo.Config()
			Expect(err).To(BeNil())

			config.User.Name = "John Doe"
			config.User.Email = "jdoe@redhat.com"

			config.Raw.AddOption("commit", "", "gpgsign", "true")
			config.Raw.AddOption("gpg", "x509", "program", "gitsign")
			config.Raw.AddOption("gpg", "", "format", "x509")

			config.Raw.AddOption("gitsign", "", "fulcio", tas.FulcioURL)
			config.Raw.AddOption("gitsign", "", "rekor", tas.RekorURL)
			config.Raw.AddOption("gitsign", "", "issuer", tas.OidcIssuerURL)

			repo.SetConfig(config)

			d1 := []byte(uuid.New().String())
			testFileName := dir + "/testFile.txt"
			Expect(os.WriteFile(testFileName, d1, 0644)).To(Succeed())
			worktree, err := repo.Worktree()
			Expect(err).To(BeNil())
			worktree.Add(".")

			token, err := testSupport.GetOIDCToken(tas.OidcIssuerURL, "jdoe@redhat.com", "secure", tas.OIDC_REALM)
			Expect(err).To(BeNil())
			Expect(gitsign.GitWithGitSign(testSupport.TestContext, dir, token, "commit", "-m", "\"CI commit "+time.Now().String()+"\"")).To(Succeed())

			// TODO: replace with tekton status check
			time.Sleep(30 * time.Second)

			Expect(repo.Push(&git.PushOptions{
				Auth: &gitAuth.BasicAuth{
					Username: "ignore",
					Password: GITHUB_TOKEN,
				}})).To(Succeed())

			ref, err := repo.Head()
			Expect(err).To(BeNil())
			logEntry, err := repo.Log(&git.LogOptions{
				From: ref.Hash(),
			})
			Expect(err).To(BeNil())
			commit, err := logEntry.Next()
			Expect(err).To(BeNil())
			Expect(commit.PGPSignature).To(Not(BeNil()))
		})

		t.Run("Verify pipeline run", func(t *testing.T) {
			// verify tekton pipeline run
			time.Sleep(5 * time.Minute)
		})
	})
}

func createTriggerBindingResource(ns string) *v1beta1.TriggerBinding {
	return &v1beta1.TriggerBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sigstore-triggerbinding",
			Namespace: ns,
		},
		Spec: v1beta1.TriggerBindingSpec{
			Params: []v1beta1.Param{
				{
					Name:  "fulcio-url",
					Value: tas.FulcioURL,
				},
				{
					Name:  "fulcio-crt-pem-url",
					Value: tas.TufURL + "/targets/fulcio_v1.crt.pem",
				},
				{
					Name:  "rekor-url",
					Value: tas.RekorURL,
				},
				{
					Name:  "issuer-url",
					Value: tas.OidcIssuerURL,
				},
				{
					Name:  "tuff-mirror",
					Value: tas.TufURL,
				},
				{
					Name:  "tuff-root",
					Value: tas.TufURL + "/root.json",
				},
				{
					Name:  "rekor-public-key",
					Value: tas.TufURL + "/targets/rekor.pub",
				},
			},
		},
	}
}
