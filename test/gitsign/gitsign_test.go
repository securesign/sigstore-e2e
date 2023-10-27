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
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
	"sigstore-e2e-test/pkg/api"
	"sigstore-e2e-test/pkg/clients"
	"sigstore-e2e-test/pkg/kubernetes"
	"sigstore-e2e-test/pkg/kubernetes/keycloak"
	"sigstore-e2e-test/pkg/kubernetes/tekton"
	"sigstore-e2e-test/pkg/support"
	"sigstore-e2e-test/pkg/tas"
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

	var sigstoreOcp = tas.NewSigstoreOcp(testSupport.TestContext)
	var keycloak = keycloak.NewOperatorInstaller(testSupport.TestContext, nil)
	var keycloakTas = tas.NewKeycloakTas(testSupport.TestContext, keycloak, sigstoreOcp, true)
	var tasHelm = tas.NewHelmInstaller(testSupport.TestContext, sigstoreOcp)
	var gitsign = clients.NewGitsign(testSupport.TestContext)
	var tekton = tekton.NewOperatorInstaller(testSupport.TestContext, nil)
	var testProject = kubernetes.NewTestProject(testSupport.TestContext, "", false)

	var oidcIssuer *url.URL
	BeforeAll(func() {
		if GithubToken == "" {
			Skip("This test require TEST_GITHUB_TOKEN provided with GitHub access token")
		}
		// prepare local prerequisites
		Expect(testSupport.InstallPrerequisites(
			sigstoreOcp,
			gitsign,
		)).To(Succeed())
		// prepare openshift prerequisites
		Expect(testSupport.InstallPrerequisites(
			keycloak,
			keycloakTas,
			tasHelm,
			tekton,
			testProject,
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
			Expect(kubernetes.K8sClient.CreateResource(testSupport.TestContext, testProject.Namespace, path+"/verify-commit-signature-task.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testSupport.TestContext, testProject.Namespace, path+"/verify-source-code-pipeline.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testSupport.TestContext, testProject.Namespace, path+"/verify-source-code-triggertemplate.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testSupport.TestContext, testProject.Namespace, path+"/verify-source-el.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testSupport.TestContext, testProject.Namespace, path+"/verify-source-el-route.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testSupport.TestContext, testProject.Namespace, path+"/webhook-secret-securesign-pipelines-demo.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testSupport.TestContext, testProject.Namespace, path+"/github-push-triggerbinding.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.Create(testSupport.TestContext, createTriggerBindingResource(testProject.Namespace))).To(Succeed())

			route := &v1.Route{}
			Eventually(func() v1.Route {
				kubernetes.K8sClient.Get(testSupport.TestContext, controller.ObjectKey{
					Namespace: testProject.Namespace,
					Name:      "el-verify-source",
				}, route)
				return *route
			}, time.Minute).Should(And(Not(BeNil()), WithTransform(func(r v1.Route) string { return r.Status.Ingress[0].Host }, Not(BeEmpty()))))
			webhookUrl = route.Status.Ingress[0].Host

			Eventually(func() []v12.Pod {
				pods, _ := kubernetes.K8sClient.CoreV1().Pods(testProject.Namespace).List(testSupport.TestContext, metav1.ListOptions{
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

				config.Raw.AddOption("gitsign", "", "fulcio", api.FulcioURL)
				config.Raw.AddOption("gitsign", "", "rekor", api.RekorURL)
				config.Raw.AddOption("gitsign", "", "issuer", oidcIssuer.String())

				Expect(repo.SetConfig(config)).To(Succeed())
			})

			It("add and push signed commit", func() {
				testFileName := dir + "/testFile.txt"
				Expect(os.WriteFile(testFileName, []byte(uuid.New().String()), 0644)).To(Succeed())
				worktree, err := repo.Worktree()
				Expect(err).To(BeNil())
				_, err = worktree.Add(".")
				Expect(err).To(BeNil())

				token, err := testSupport.GetOIDCToken(api.OidcIssuerURL, "jdoe@redhat.com", "secure", tas.OIDC_REALM)
				Expect(err).To(BeNil())
				Expect(token).To(Not(BeEmpty()))

				Expect(gitsign.GitWithGitSign(dir, token, "commit", "-m", "CI commit "+time.Now().String())).To(Succeed())

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
					kubernetes.K8sClient.List(testSupport.TestContext, pipelineRuns,
						controller.InNamespace(testProject.Namespace),
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
			Name:      "sigstore-triggerbinding",
			Namespace: ns,
		},
		Spec: v1beta1.TriggerBindingSpec{
			Params: []v1beta1.Param{
				{
					Name:  "fulcio-url",
					Value: api.FulcioURL,
				},
				{
					Name:  "fulcio-crt-pem-url",
					Value: api.TufURL + "/targets/fulcio-cert",
				},
				{
					Name:  "rekor-url",
					Value: api.RekorURL,
				},
				{
					Name:  "issuer-url",
					Value: api.OidcIssuerURL,
				},
				{
					Name:  "tuff-mirror",
					Value: api.TufURL,
				},
				{
					Name:  "tuff-root",
					Value: api.TufURL + "/root.json",
				},
				{
					Name:  "rekor-public-key",
					Value: api.TufURL + "/targets/rekor-pubkey",
				},
				{
					Name:  "ctfe-public-key",
					Value: api.TufURL + "/targets/ctfe.pub",
				},
			},
		},
	}
}
