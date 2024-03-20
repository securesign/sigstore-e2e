package gitsign

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/securesign/sigstore-e2e/test/testsupport"

	"github.com/securesign/sigstore-e2e/pkg/clients"

	"github.com/securesign/sigstore-e2e/pkg/api"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var logIndex string
var hashValue string
var tempDir string
var publicKeyPath string
var signaturePath string

type RekorCLIOutput struct {
	HashedRekordObj struct {
		Data struct {
			Hash struct {
				Value string `json:"value"`
			} `json:"hash"`
		} `json:"data"`
		Signature struct {
			Content   string `json:"content"`
			PublicKey struct {
				Content string `json:"content"`
			} `json:"publicKey"`
		} `json:"signature"`
	} `json:"HashedRekordObj"`
}

var _ = Describe("Signing and verifying commits by using Gitsign from the command-line interface", Ordered, func() {
	var gitsign = clients.NewGitsign()
	var cosign = clients.NewCosign()
	var rekorCli = clients.NewRekorCli()

	var (
		dir    string
		config *config.Config
		repo   *git.Repository
		err    error
	)
	BeforeAll(func() {
		err = testsupport.CheckAnyTestMandatoryAPIConfigValues()
		if err != nil {
			Skip("Skip this test - " + err.Error())
		}

		Expect(testsupport.InstallPrerequisites(
			gitsign,
			cosign,
			rekorCli,
		)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

		tempDir, err = os.MkdirTemp("", "rekorTest")
		Expect(err).ToNot(HaveOccurred())

		// initialize local git repository
		dir, err = os.MkdirTemp("", "repository")
		Expect(err).ToNot(HaveOccurred())
		repo, err = git.PlainInit(dir, false)
		Expect(err).ToNot(HaveOccurred())
		config, err = repo.Config()
		Expect(err).ToNot(HaveOccurred())
	})

	Context("With configured git", func() {
		It("sets the local repository to use 'jdoe@redhat.com' user", func() {
			config.User.Name = "John Doe"
			config.User.Email = "jdoe@redhat.com"

			Expect(repo.SetConfig(config)).To(Succeed())
		})

		It("configures the local repository configuration to sign your commits by using the TAS service", func() {
			config.Raw.AddOption("commit", "", "gpgsign", "true")
			config.Raw.AddOption("tag", "", "gpgsign", "true")
			config.Raw.AddOption("gpg", "x509", "program", "gitsign")
			config.Raw.AddOption("gpg", "", "format", "x509")
			config.Raw.AddOption("gitsign", "", "fulcio", api.GetValueFor(api.FulcioURL))
			config.Raw.AddOption("gitsign", "", "rekor", api.GetValueFor(api.RekorURL))
			config.Raw.AddOption("gitsign", "", "issuer", api.GetValueFor(api.OidcIssuerURL))

			Expect(repo.SetConfig(config)).To(Succeed())
		})
	})

	Describe("Make a commit to the local repository", func() {
		It("creates a new file and stage it", func() {
			testFileName := dir + "/testFile.txt"
			Expect(os.WriteFile(testFileName, []byte(uuid.New().String()), 0600)).To(Succeed())
			worktree, err := repo.Worktree()
			Expect(err).ToNot(HaveOccurred())
			_, err = worktree.Add(".")
			Expect(err).ToNot(HaveOccurred())
		})

		It("gets ID token and makes commit", func() {
			token, err := testsupport.GetOIDCToken(testsupport.TestContext, api.GetValueFor(api.OidcIssuerURL),
				"jdoe@redhat.com",
				"secure",
				api.GetValueFor(api.OidcRealm))
			Expect(err).ToNot(HaveOccurred())
			Expect(token).To(Not(BeEmpty()))
			Expect(gitsign.GitWithGitSign(testsupport.TestContext, dir, token, "commit", "-S", "-m", "CI commit "+time.Now().String())).To(Succeed())
		})

		It("checks that commit has PGP signature", func() {
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

	Describe("Verify the commit", func() {
		Context("With initialized Fulcio CA", func() {
			It("initialize cosign", func() {
				Expect(cosign.Command(testsupport.TestContext, "initialize").Run()).To(Succeed())
			})
		})

		When("commiter is authorized", func() {
			It("should verify HEAD signature by gitsign", func() {
				cmd := gitsign.Command(testsupport.TestContext, "verify",
					"--certificate-identity", "jdoe@redhat.com",
					"--certificate-oidc-issuer", api.GetValueFor(api.OidcIssuerURL),
					"HEAD")

				cmd.Dir = dir

				// gitsign requires to find git in PATH
				cmd.Env = os.Environ()

				var output bytes.Buffer

				cmd.Stdout = &output

				Expect(cmd.Run()).To(Succeed())
				logrus.Info(output.String())

				re := regexp.MustCompile(`tlog index: (\d+)`)
				match := re.FindStringSubmatch(output.String())

				logIndex = match[1]
			})
		})
	})

	Describe("rekor-cli get with logIndex", func() {
		It("should retrieve the entry from Rekor", func() {
			// Assuming `logIndex` is obtained from previous tests or steps
			rekorServerURL := api.GetValueFor(api.RekorURL)

			output, err := rekorCli.CommandOutput(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--log-index", logIndex)
			Expect(err).ToNot(HaveOccurred())

			logrus.Info(string(output))

			startIndex := strings.Index(string(output), "{")
			if startIndex == -1 {
				// Handle error: JSON start not found
				return
			}
			jsonStr := string(output[startIndex:])

			var result RekorCLIOutput
			err = json.Unmarshal([]byte(jsonStr), &result)
			Expect(err).ToNot(HaveOccurred())

			signatureContent := result.HashedRekordObj.Signature.Content
			publicKeyContent := result.HashedRekordObj.Signature.PublicKey.Content
			hashValue = result.HashedRekordObj.Data.Hash.Value

			decodedSignatureContent, err := base64.StdEncoding.DecodeString(signatureContent)
			Expect(err).ToNot(HaveOccurred())

			// Decode publicKeyContent from base64
			decodedPublicKeyContent, err := base64.StdEncoding.DecodeString(publicKeyContent)
			Expect(err).ToNot(HaveOccurred())

			publicKeyPath = filepath.Join(tempDir, "publickey.pem")
			signaturePath = filepath.Join(tempDir, "signature.bin")

			Expect(os.WriteFile(publicKeyPath, decodedPublicKeyContent, 0600)).To(Succeed())
			Expect(os.WriteFile(signaturePath, decodedSignatureContent, 0600)).To(Succeed())

		})
	})

	Describe("Rekor CLI Verify Artifact", func() {
		It("should verify the artifact using rekor-cli", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL) // Ensure this is the correct way to retrieve your Rekor server URL
			// Ensure hashValue, signature.bin, and publickey.pem are available and correctly set up before this step.
			Expect(rekorCli.Command(testsupport.TestContext, "verify", "--rekor_server", rekorServerURL, "--signature", signaturePath, "--public-key", publicKeyPath, "--pki-format", "x509", "--type", "hashedrekord:0.0.1", "--artifact-hash", hashValue).Run()).To(Succeed())
		})
	})
})

var _ = AfterSuite(func() {
	// Cleanup shared resources after all tests have run.
	Expect(os.RemoveAll(tempDir)).To(Succeed())
})
