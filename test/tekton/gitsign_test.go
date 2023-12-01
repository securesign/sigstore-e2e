package tekton

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sigstore-e2e-test/pkg/api"
	"sigstore-e2e-test/pkg/clients"
	"sigstore-e2e-test/pkg/kubernetes"
	"sigstore-e2e-test/pkg/support"
	"sigstore-e2e-test/test/testsupport"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	gitAuth "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v56/github"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	v1beta12 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
	v12 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("gitsign test", Ordered, func() {
	var (
		GithubToken    = api.GetValueFor(api.GithubToken)
		GithubUsername = api.GetValueFor(api.GithubUsername)
		GithubOwner    = api.GetValueFor(api.GithubOwner)
		GithubRepo     = api.GetValueFor(api.GithubRepo)
		webhookURL     string
		err            error
		githubClient   *github.Client
		webhook        *github.Hook
		gitsign        *clients.Gitsign
		testProject    *kubernetes.ProjectPrerequisite
	)

	BeforeAll(func() {
		err = testsupport.CheckAPIConfigValues(testsupport.Mandatory, api.GithubToken, api.GithubUsername, api.GithubOwner, api.GithubRepo,
			api.FulcioURL, api.RekorURL, api.OidcIssuerURL, api.TufURL, api.OidcRealm)
		if err != nil {
			Skip("Skip this test - " + err.Error())
		}

		githubClient = github.NewClient(nil).WithAuthToken(GithubToken)
		gitsign = clients.NewGitsign()
		testProject = kubernetes.NewTestProject("", false)

		Expect(testsupport.InstallPrerequisites(
			gitsign,
			testProject,
		)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})
	})

	AfterAll(func() {
		if webhook != nil {
			_, _ = githubClient.Repositories.DeleteHook(testsupport.TestContext, GithubOwner, GithubRepo, *webhook.ID)
		}
	})

	Describe("Install tekton pipelines", func() {
		It("create tekton ENV", func() {
			// use file definition for large tekton resources
			_, b, _, _ := runtime.Caller(0)
			path := filepath.Dir(b) + "/resources"
			Expect(kubernetes.K8sClient.CreateResource(testsupport.TestContext, testProject.Namespace, path+"/verify-commit-signature-task.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testsupport.TestContext, testProject.Namespace, path+"/verify-source-code-pipeline.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testsupport.TestContext, testProject.Namespace, path+"/verify-source-code-triggertemplate.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testsupport.TestContext, testProject.Namespace, path+"/verify-source-el.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testsupport.TestContext, testProject.Namespace, path+"/verify-source-el-route.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testsupport.TestContext, testProject.Namespace, path+"/webhook-secret-securesign-pipelines-demo.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testsupport.TestContext, testProject.Namespace, path+"/github-push-triggerbinding.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.CreateResource(testsupport.TestContext, testProject.Namespace, path+"/verify-source-code-triggerbinding.yaml")).To(Succeed())
			Expect(kubernetes.K8sClient.Create(testsupport.TestContext, createTriggerBindingResource(testProject.Namespace))).To(Succeed())

			route := &v1.Route{}
			Eventually(func() v1.Route {
				_ = kubernetes.K8sClient.Get(testsupport.TestContext, controller.ObjectKey{
					Namespace: testProject.Namespace,
					Name:      "el-verify-source",
				}, route)
				return *route
			}, time.Minute).Should(And(Not(BeNil()), WithTransform(func(r v1.Route) string { return r.Status.Ingress[0].Host }, Not(BeEmpty()))))
			webhookURL = route.Status.Ingress[0].Host

			Eventually(func() []v12.Pod {
				pods, _ := kubernetes.K8sClient.CoreV1().Pods(testProject.Namespace).List(testsupport.TestContext, metav1.ListOptions{
					LabelSelector: "eventlistener=verify-source"})
				return pods.Items
			}, testsupport.TestTimeoutMedium).Should(And(HaveLen(1), WithTransform(func(pods []v12.Pod) v12.PodPhase { return pods[0].Status.Phase }, Equal(v12.PodRunning))))
		})
		// sleep a few more seconds for everything to settle down
		time.Sleep(30 * time.Second)
	})

	Describe("register GitHub webhook", func() {
		It("register tekton webhook", func() {
			hookConfig := make(map[string]interface{})
			hookConfig["url"] = "https://" + webhookURL
			hookConfig["content_type"] = "json"
			hookConfig["secret"] = "secretToken"
			hookName := "test" + uuid.New().String()

			var (
				response *github.Response
				err      error
			)
			webhook, response, err = githubClient.Repositories.CreateHook(testsupport.TestContext, GithubOwner, GithubRepo, &github.Hook{
				Name:   &hookName,
				Config: hookConfig,
				Events: []string{"push"},
			})
			Expect(err).ToNot(HaveOccurred())
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
				Expect(err).ToNot(HaveOccurred())
				config.User.Name = "John Doe"
				config.User.Email = "jdoe@redhat.com"

				config.Raw.AddOption("commit", "", "gpgsign", "true")
				config.Raw.AddOption("gpg", "x509", "program", "gitsign")
				config.Raw.AddOption("gpg", "", "format", "x509")

				config.Raw.AddOption("gitsign", "", "fulcio", api.GetValueFor(api.FulcioURL))
				config.Raw.AddOption("gitsign", "", "rekor", api.GetValueFor(api.RekorURL))
				config.Raw.AddOption("gitsign", "", "issuer", api.GetValueFor(api.OidcIssuerURL))

				Expect(repo.SetConfig(config)).To(Succeed())
			})

			It("add and push signed commit", func() {
				testFileName := dir + "/testFile.txt"
				Expect(os.WriteFile(testFileName, []byte(uuid.New().String()), 0600)).To(Succeed())
				worktree, err := repo.Worktree()
				Expect(err).ToNot(HaveOccurred())
				_, err = worktree.Add(".")
				Expect(err).ToNot(HaveOccurred())

				token, err := testsupport.GetOIDCToken(testsupport.TestContext, api.GetValueFor(api.OidcIssuerURL), "jdoe@redhat.com", "secure", api.GetValueFor(api.OidcRealm))
				Expect(err).ToNot(HaveOccurred())
				Expect(token).To(Not(BeEmpty()))

				Expect(gitsign.GitWithGitSign(testsupport.TestContext, dir, token, "commit", "-m", "CI commit "+time.Now().String())).To(Succeed())

				Expect(repo.Push(&git.PushOptions{
					Auth: &gitAuth.BasicAuth{
						Username: GithubUsername,
						Password: GithubToken,
					}})).To(Succeed())

				ref, err := repo.Head()
				Expect(err).ToNot(HaveOccurred())
				logEntry, err := repo.Log(&git.LogOptions{
					From: ref.Hash(),
				})
				Expect(err).ToNot(HaveOccurred())
				commit, err := logEntry.Next()
				Expect(err).ToNot(HaveOccurred())
				Expect(commit.PGPSignature).To(Not(BeNil()))
			})
		})

		Describe("verify pipeline run was executed", func() {
			It("pipeline run is successful", func() {

				Eventually(func() []v1beta12.PipelineRun {
					pipelineRuns := &v1beta12.PipelineRunList{}
					Expect(kubernetes.K8sClient.List(testsupport.TestContext, pipelineRuns,
						controller.InNamespace(testProject.Namespace),
						controller.MatchingLabels{"tekton.dev/pipeline": "verify-source-code-pipeline"},
					)).ToNot(HaveOccurred())
					return pipelineRuns.Items
				}, testsupport.TestTimeoutMedium).Should(And(HaveLen(1), WithTransform(func(list []v1beta12.PipelineRun) bool {
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
					Value: api.GetValueFor(api.FulcioURL),
				},
				{
					Name:  "fulcio-crt-pem-url",
					Value: api.GetValueFor(api.TufURL) + "/targets/fulcio-cert",
				},
				{
					Name:  "rekor-url",
					Value: api.GetValueFor(api.RekorURL),
				},
				{
					Name:  "issuer-url",
					Value: api.GetValueFor(api.OidcIssuerURL),
				},
				{
					Name:  "tuff-mirror",
					Value: api.GetValueFor(api.TufURL),
				},
				{
					Name:  "tuff-root",
					Value: api.GetValueFor(api.TufURL) + "/root.json",
				},
				{
					Name:  "rekor-public-key",
					Value: api.GetValueFor(api.TufURL) + "/targets/rekor-pubkey",
				},
				{
					Name:  "ctfe-public-key",
					Value: api.GetValueFor(api.TufURL) + "/targets/ctfe.pub",
				},
			},
		},
	}
}
