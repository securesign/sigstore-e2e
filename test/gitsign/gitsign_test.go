package gitsign

import (
	"encoding/json"
	"fmt"
	git "github.com/go-git/go-git/v5"
	gitAuth "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v56/github"
	"github.com/google/uuid"
	"github.com/onsi/gomega"
	v1 "github.com/openshift/api/route/v1"
	"github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigstore-e2e-test/test/support"
	"strconv"
	"strings"
	"testing"
	"time"
)

const GITHUB_TOKEN = "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"
const GITHUB_OWNER = "bouskaJ"
const GITHUB_REPO = "gitsign-demo-test"

func TestSignVerifyCommit(t *testing.T) {
	support.WithNewTestNamespace(func(ns string) {
		gomega.RegisterTestingT(t)

		// use file definition for large tekton resources
		_, b, _, _ := runtime.Caller(0)
		path := filepath.Dir(b) + "/resources"
		createResource(ns, path+"/verify-commit-signature-task.yaml")
		createResource(ns, path+"/verify-source-code-pipeline.yaml")
		createResource(ns, path+"/verify-source-code-triggertemplate.yaml")
		createResource(ns, path+"/verify-source-el.yaml")
		createResource(ns, path+"/verify-source-el-route.yaml")
		createResource(ns, path+"/webhook-secret-securesign-pipelines-demo.yaml")
		createResource(ns, path+"/github-push-triggerbinding.yaml")
		gomega.Expect(support.TestClient.Create(support.TestContext, createTriggerBindingResource(ns))).To(gomega.Succeed())

		route := &v1.Route{}
		gomega.Eventually(func() v1.Route {
			support.TestClient.Get(support.TestContext, client.ObjectKey{
				Namespace: ns,
				Name:      "el-verify-source",
			}, route)
			return *route
		}, time.Minute).Should(gomega.Not(gomega.BeNil()))
		gomega.Eventually(route.Status.Ingress[0].Host).Should(gomega.Not(gomega.BeNil()))

		// register webhook
		// TODO: migrate somewhere else
		hookConfig := make(map[string]interface{})
		hookConfig["url"] = "https://" + route.Status.Ingress[0].Host
		hookConfig["content_type"] = "json"
		hookConfig["secret"] = "secretToken"
		hookName := "test" + uuid.New().String()
		client := github.NewClient(nil).WithAuthToken(GITHUB_TOKEN)
		hook, response, err := client.Repositories.CreateHook(support.TestContext, GITHUB_OWNER, GITHUB_REPO, &github.Hook{
			Name:   &hookName,
			Config: hookConfig,
			Events: []string{"push"},
		})
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(response.Status).To(gomega.Equal("201 Created"))
		defer client.Repositories.DeleteHook(support.TestContext, GITHUB_OWNER, GITHUB_REPO, *hook.ID)

		t.Run("Sign github commit", func(t *testing.T) {
			dir, _ := os.MkdirTemp("", "sigstore")
			repo, err := git.PlainClone(dir, false, &git.CloneOptions{
				URL:      fmt.Sprintf("https://github.com/%s/%s.git", GITHUB_OWNER, GITHUB_REPO),
				Progress: os.Stdout,
			})
			gomega.Expect(err).To(gomega.BeNil())

			config, err := repo.Config()
			gomega.Expect(err).To(gomega.BeNil())

			config.User.Name = "John Doe"
			config.User.Email = "jdoe@redhat.com"

			config.Raw.AddOption("commit", "", "gpgsign", "true")
			config.Raw.AddOption("gpg", "x509", "program", "gitsign")
			config.Raw.AddOption("gpg", "", "format", "x509")

			config.Raw.AddOption("gitsign", "", "fulcio", os.Getenv("FULCIO_URL"))
			config.Raw.AddOption("gitsign", "", "rekor", os.Getenv("REKOR_URL"))
			config.Raw.AddOption("gitsign", "", "issuer", os.Getenv("OIDC_ISSUER_URL"))

			repo.SetConfig(config)

			d1 := []byte(uuid.New().String())
			testFileName := dir + "/testFile.txt"
			gomega.Expect(os.WriteFile(testFileName, d1, 0644)).To(gomega.Succeed())
			worktree, err := repo.Worktree()
			gomega.Expect(err).To(gomega.BeNil())
			worktree.Add(".")

			token, err := getOIDCToken()
			gomega.Expect(err).To(gomega.BeNil())

			// use native git & gitsign commands (go-git Commit does not execute commit)
			cmd := exec.CommandContext(support.TestContext, "/bin/sh", "-c", "git commit -m \"CI commit "+time.Now().String()+"\"")
			gitsignPath, err := exec.LookPath("gitsign")
			gitPath, err := exec.LookPath("git")
			cmd.Env = append(cmd.Env, "SIGSTORE_ID_TOKEN="+token, "PATH=$PATH:"+filepath.Dir(gitsignPath)+":"+filepath.Dir(gitPath))
			cmd.Dir = dir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stdout
			err = cmd.Run()
			gomega.Expect(err).To(gomega.BeNil())

			// TODO: replace with tekton status check
			time.Sleep(30 * time.Second)

			gomega.Expect(repo.Push(&git.PushOptions{
				Auth: &gitAuth.BasicAuth{
					Username: "ignore",
					Password: GITHUB_TOKEN,
				}})).To(gomega.Succeed())

			ref, err := repo.Head()
			gomega.Expect(err).To(gomega.BeNil())
			logEntry, err := repo.Log(&git.LogOptions{
				From: ref.Hash(),
			})
			gomega.Expect(err).To(gomega.BeNil())
			commit, err := logEntry.Next()
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(commit.PGPSignature).To(gomega.Not(gomega.BeNil()))
		})

		t.Run("Verify pipeline run", func(t *testing.T) {
			// verify tekton pipeline run
			time.Sleep(5 * time.Minute)
		})
	})
}

func createResource(ns string, filePath string) {
	byte, _ := os.ReadFile(filePath)
	object := &unstructured.Unstructured{}
	gomega.Expect(yaml.Unmarshal(byte, object)).To(gomega.Succeed())
	object.SetNamespace(ns)
	gomega.Expect(support.TestClient.Create(support.TestContext, object)).To(gomega.Succeed())
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
					Value: os.Getenv("FULCIO_URL"),
				},
				{
					Name:  "fulcio-crt-pem-url",
					Value: os.Getenv("TUF_URL") + "/targets/fulcio_v1.crt.pem",
				},
				{
					Name:  "rekor-url",
					Value: os.Getenv("REKOR_URL"),
				},
				{
					Name:  "issuer-url",
					Value: os.Getenv("OIDC_ISSUER_URL"),
				},
				{
					Name:  "tuff-mirror",
					Value: os.Getenv("TUF_URL"),
				},
				{
					Name:  "tuff-root",
					Value: os.Getenv("TUF_URL") + "/root.json",
				},
				{
					Name:  "rekor-public-key",
					Value: os.Getenv("TUF_URL") + "/targets/rekor.pub",
				},
			},
		},
	}
}

func getOIDCToken() (string, error) {
	urlString := os.Getenv("OIDC_ISSUER_URL") + "/protocol/openid-connect/token"

	client := &http.Client{}
	data := url.Values{}
	data.Set("username", "jdoe@redhat.com")
	data.Set("password", "secure")
	data.Set("scope", "openid")
	data.Set("client_id", "sigstore")
	data.Set("grant_type", "password")

	r, _ := http.NewRequest(http.MethodPost, urlString, strings.NewReader(data.Encode())) // URL-encoded payload
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	resp, err := client.Do(r)
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(resp.Body)

	defer resp.Body.Close()
	if err != nil {
		return "", err
	}
	jsonOut := make(map[string]interface{})
	err = json.Unmarshal(b, &jsonOut)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", jsonOut["access_token"]), nil
}
