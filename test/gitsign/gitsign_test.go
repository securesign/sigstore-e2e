package gitsign

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	gitAuth "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v56/github"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/route/v1"
	v1beta12 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
	v12 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path/filepath"
	"runtime"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
	"sigstore-e2e-test/pkg/support"
	"sigstore-e2e-test/pkg/tas"
	"sigstore-e2e-test/pkg/tas/gitsign"
	"sigstore-e2e-test/pkg/tekton"
	"sigstore-e2e-test/test/testSupport"
	"time"
)

var (
	GithubToken    = os.Getenv("TEST_GITHUB_TOKEN")
	GithubUsername = support.GetEnvOrDefault("TEST_GITHUB_USER", "ignore")
	GithubOwner    = support.GetEnvOrDefault("TEST_GITHUB_OWNER", "securesign")
	GithubRepo     = support.GetEnvOrDefault("TEST_GITHUB_REPO", "e2e-gitsign-test")
)

var _ = Describe("gitsign test", Ordered, func() {
	var webhookUrl string
	githubClient := github.NewClient(nil).WithAuthToken(GithubToken)
	var webhook *github.Hook
	BeforeAll(func() {
		if GithubToken == "" {
			Skip("This test require TEST_GITHUB_TOKEN provided with GitHub access token")
		}
		Expect(testSupport.InstallPrerequisites(
			tas.NewTas(testSupport.TestContext),
			gitsign.NewGitsignInstaller(testSupport.TestContext),
			tekton.NewTektonInstaller(testSupport.TestContext),
			support.NewTestProject(testSupport.TestContext),
		)).To(Succeed())
		DeferCleanup(func() { testSupport.DestroyPrerequisites() })
	})

	AfterAll(func() {
		if webhook != nil {
			_, _ = githubClient.Repositories.DeleteHook(testSupport.TestContext, GithubOwner, GithubRepo, *webhook.ID)
		}
	})

	Describe("Install tekton pipelines", func() {
		It("create tekton ENV", func() {
			// use file definition for large tekton resources
			_, b, _, _ := runtime.Caller(0)
			path := filepath.Dir(b) + "/resources"
			Expect(testSupport.TestClient.CreateResource(testSupport.TestContext, support.TestNamespace, path+"/verify-commit-signature-task.yaml")).To(Succeed())
			Expect(testSupport.TestClient.CreateResource(testSupport.TestContext, support.TestNamespace, path+"/verify-source-code-pipeline.yaml")).To(Succeed())
			Expect(testSupport.TestClient.CreateResource(testSupport.TestContext, support.TestNamespace, path+"/verify-source-code-triggertemplate.yaml")).To(Succeed())
			Expect(testSupport.TestClient.CreateResource(testSupport.TestContext, support.TestNamespace, path+"/verify-source-el.yaml")).To(Succeed())
			Expect(testSupport.TestClient.CreateResource(testSupport.TestContext, support.TestNamespace, path+"/verify-source-el-route.yaml")).To(Succeed())
			Expect(testSupport.TestClient.CreateResource(testSupport.TestContext, support.TestNamespace, path+"/webhook-secret-securesign-pipelines-demo.yaml")).To(Succeed())
			Expect(testSupport.TestClient.CreateResource(testSupport.TestContext, support.TestNamespace, path+"/github-push-triggerbinding.yaml")).To(Succeed())
			Expect(testSupport.TestClient.CreateResource(testSupport.TestContext, support.TestNamespace, path+"/verify-source-code-triggerbinding.yaml")).To(Succeed())
			Expect(testSupport.TestClient.Create(testSupport.TestContext, createTriggerBindingResource(support.TestNamespace))).To(Succeed())

			route := &v1.Route{}
			Eventually(func() v1.Route {
				testSupport.TestClient.Get(testSupport.TestContext, controller.ObjectKey{
					Namespace: support.TestNamespace,
					Name:      "el-verify-source",
				}, route)
				return *route
			}, time.Minute).Should(And(Not(BeNil()), WithTransform(func(r v1.Route) string { return r.Status.Ingress[0].Host }, Not(BeEmpty()))))
			webhookUrl = route.Status.Ingress[0].Host

			Eventually(func() []v12.Pod {
				pods, _ := testSupport.TestClient.CoreV1().Pods(support.TestNamespace).List(testSupport.TestContext, metav1.ListOptions{
					LabelSelector: "eventlistener=verify-source"})
				return pods.Items
			}, testSupport.TestTimeoutMedium).Should(And(HaveLen(1), WithTransform(func(pods []v12.Pod) v12.PodPhase { return pods[0].Status.Phase }, Equal(v12.PodRunning))))
		})
		// sleep a few more seconds for everything to settle down
		time.Sleep(30 * time.Second)
	})

	Describe("register GitHub webhook", func() {
		It("register tekton webhook", func() {
			hookConfig := make(map[string]interface{})
			hookConfig["url"] = "https://" + webhookUrl
			hookConfig["content_type"] = "json"
			hookConfig["secret"] = "secretToken"
			hookName := "test" + uuid.New().String()

			var (
				response *github.Response
				err      error
			)
			webhook, response, err = githubClient.Repositories.CreateHook(testSupport.TestContext, GithubOwner, GithubRepo, &github.Hook{
				Name:   &hookName,
				Config: hookConfig,
				Events: []string{"push"},
			})
			Expect(err).To(BeNil())
			Expect(response.Status).To(Equal("201 Created"))
		})

		Describe("Sign github commit", func() {
			var (
				dir    string
				config *config.Config
				repo   *git.Repository
				err    error
			)
			Context("with git repository", func() {
				dir, repo, err = support.GitCloneWithAuth(fmt.Sprintf("https://github.com/%s/%s.git", GithubOwner, GithubRepo),
					&gitAuth.BasicAuth{
						Username: GithubUsername,
						Password: GithubToken,
					})
			})

			It("Add git configuration for gitsign", func() {
				config, err = repo.Config()
				Expect(err).To(BeNil())
				config.User.Name = "John Doe"
				config.User.Email = "jdoe@redhat.com"

				config.Raw.AddOption("commit", "", "gpgsign", "true")
				config.Raw.AddOption("gpg", "x509", "program", "gitsign")
				config.Raw.AddOption("gpg", "", "format", "x509")

				config.Raw.AddOption("gitsign", "", "fulcio", tas.FulcioURL)
				config.Raw.AddOption("gitsign", "", "rekor", tas.RekorURL)
				config.Raw.AddOption("gitsign", "", "issuer", tas.OidcIssuerURL)

				Expect(repo.SetConfig(config)).To(Succeed())
			})

			It("add and push signed commit", func() {
				testFileName := dir + "/testFile.txt"
				Expect(os.WriteFile(testFileName, []byte(uuid.New().String()), 0644)).To(Succeed())
				worktree, err := repo.Worktree()
				Expect(err).To(BeNil())
				_, err = worktree.Add(".")
				Expect(err).To(BeNil())

				token, err := testSupport.GetOIDCToken(tas.OidcIssuerURL, "jdoe@redhat.com", "secure", tas.OIDC_REALM)
				Expect(err).To(BeNil())
				Expect(token).To(Not(BeEmpty()))

				Expect(gitsign.GitWithGitSign(testSupport.TestContext, dir, token, "commit", "-m", "CI commit "+time.Now().String())).To(Succeed())

				Expect(repo.Push(&git.PushOptions{
					Auth: &gitAuth.BasicAuth{
						Username: GithubUsername,
						Password: GithubToken,
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
		})

		Describe("verify pipeline run was executed", func() {
			It("pipeline run is successful", func() {

				Eventually(func() []v1beta12.PipelineRun {
					pipelineRuns := &v1beta12.PipelineRunList{}
					testSupport.TestClient.List(testSupport.TestContext, pipelineRuns,
						controller.InNamespace(support.TestNamespace),
						controller.MatchingLabels{"tekton.dev/pipeline": "verify-source-code-pipeline"},
					)
					return pipelineRuns.Items
				}, testSupport.TestTimeoutMedium).Should(And(HaveLen(1), WithTransform(func(list []v1beta12.PipelineRun) bool {
					return list[0].Status.GetCondition("Succeeded").IsTrue()
				}, BeTrue())))

			})
		})

	})
})

func createTriggerBindingResource(ns string) *v1beta1.TriggerBinding {
	return &v1beta1.TriggerBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trusted-artifact-signer-triggerbinding",
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
					Value: tas.TufURL + "/targets/fulcio-cert",
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
					Value: tas.TufURL + "/targets/rekor-pubkey",
				},
				{
					Name:  "ctfe-public-key",
					Value: tas.TufURL + "/targets/ctfe.pub",
				},
			},
		},
	}
}
