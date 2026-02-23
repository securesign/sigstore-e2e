package gitsign

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
var rekorEntryType string
var tempDir string
var publicKeyPath string
var signaturePath string

var _ = Describe("Signing and verifying commits by using Gitsign from the command-line interface", Ordered, func() {
	var gitsign = clients.NewGitsign()
	var rekorCli = clients.NewRekorCli()

	var (
		dir    string
		config *config.Config
		repo   *git.Repository
		err    error
	)
	BeforeAll(func() {
		err = testsupport.CheckMandatoryAPIConfigValues(api.OidcIssuerURL, api.RekorURL, api.TufURL)
		if err != nil {
			Fail(err.Error())
		}

		Expect(testsupport.InstallPrerequisites(
			gitsign,
			rekorCli,
		)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

		// tempDir for publickey and signature
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

	Describe("Gitsign initialize", func() {
		It("should initialize the TUF root", func() {
			tufURL := api.GetValueFor(api.TufURL)
			Expect(gitsign.Command(testsupport.TestContext, "initialize",
				"--mirror", tufURL,
				"--root", tufURL+"/root.json").Run()).To(Succeed())
		})
	})

	Context("With configured git", func() {
		It("sets the local repository to use OIDC user", func() {
			config.User.Name = "John Doe"
			config.User.Email = fmt.Sprintf("%s@%s", api.GetValueFor(api.OidcUser), api.GetValueFor(api.OidcUserDomain))

			Expect(repo.SetConfig(config)).To(Succeed())
		})

		It("configures the local repository to sign commits with gitsign", func() {
			config.Raw.AddOption("commit", "", "gpgsign", "true")
			config.Raw.AddOption("tag", "", "gpgsign", "true")
			config.Raw.AddOption("gpg", "x509", "program", "gitsign")
			config.Raw.AddOption("gpg", "", "format", "x509")

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
			token, err := testsupport.GetOIDCToken(testsupport.TestContext)
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
		When("commiter is authorized", func() {
			It("should verify HEAD signature by gitsign", func() {
				cmd := gitsign.Command(testsupport.TestContext, "verify",
					"--certificate-identity", fmt.Sprintf("%s@%s", api.GetValueFor(api.OidcUser), api.GetValueFor(api.OidcUserDomain)),
					"--certificate-oidc-issuer", api.GetValueFor(api.OidcIssuerURL),
					"HEAD")

				cmd.Dir = dir

				// gitsign requires to find git in PATH
				cmd.Env = os.Environ()

				var output bytes.Buffer

				cmd.Stdout = &output
				Expect(cmd.Run()).To(Succeed())
				logrus.WithField("app", "gitsign").Info(output.String())

				re := regexp.MustCompile(`tlog index: (\d+)`)
				match := re.FindStringSubmatch(output.String())

				logIndex = match[1]
			})
		})
	})

	Describe("rekor-cli get with logIndex", func() {
		It("should retrieve the entry from Rekor", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)

			output, err := rekorCli.CommandOutput(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--log-index", logIndex)
			Expect(err).ToNot(HaveOccurred())

			// Look for JSON start
			startIndex := strings.Index(string(output), "{")
			Expect(startIndex).NotTo(Equal(-1), "JSON start - '{' not found")

			jsonStr := string(output[startIndex:])

			var rekorGetOutput testsupport.RekorCLIGetOutput
			err = json.Unmarshal([]byte(jsonStr), &rekorGetOutput)
			Expect(err).ToNot(HaveOccurred())

			// Extract values from rekor-cli get output - handle both HashedRekordObj and DSSEObj
			var signatureContent, publicKeyContent string
			if len(rekorGetOutput.DSSEObj.Signatures) > 0 {
				rekorEntryType = "dsse:0.0.1"
				signatureContent = rekorGetOutput.DSSEObj.Signatures[0].Signature
				publicKeyContent = rekorGetOutput.DSSEObj.Signatures[0].Verifier
				hashValue = rekorGetOutput.DSSEObj.PayloadHash.Value
			} else if rekorGetOutput.HashedRekordObj.Signature.Content != "" {
				rekorEntryType = "hashedrekord:0.0.1"
				signatureContent = rekorGetOutput.HashedRekordObj.Signature.Content
				publicKeyContent = rekorGetOutput.HashedRekordObj.Signature.PublicKey.Content
				hashValue = rekorGetOutput.HashedRekordObj.Data.Hash.Value
			} else if rekorGetOutput.RekordObj.Signature.Content != "" {
				rekorEntryType = "rekord:0.0.1"
				signatureContent = rekorGetOutput.RekordObj.Signature.Content
				publicKeyContent = rekorGetOutput.RekordObj.Signature.PublicKey.Content
				hashValue = rekorGetOutput.RekordObj.Data.Hash.Value
			} else {
				Fail("Unrecognized Rekor entry type in rekor-cli get output")
			}

			// Decode signatureContent and publicKeyContent from base64
			decodedSignatureContent, err := base64.StdEncoding.DecodeString(signatureContent)
			Expect(err).ToNot(HaveOccurred())

			decodedPublicKeyContent, err := base64.StdEncoding.DecodeString(publicKeyContent)
			Expect(err).ToNot(HaveOccurred())

			// Create files in the tempDir
			publicKeyPath = filepath.Join(tempDir, "publickey.pem")
			signaturePath = filepath.Join(tempDir, "signature.bin")

			Expect(os.WriteFile(publicKeyPath, decodedPublicKeyContent, 0600)).To(Succeed())
			Expect(os.WriteFile(signaturePath, decodedSignatureContent, 0600)).To(Succeed())

		})
	})

	Describe("Rekor CLI Verify Artifact", func() {
		It("should verify the artifact using rekor-cli", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			Expect(rekorCli.Command(testsupport.TestContext, "verify", "--rekor_server", rekorServerURL, "--signature", signaturePath, "--public-key", publicKeyPath, "--pki-format", "x509", "--type", rekorEntryType, "--artifact-hash", hashValue).Run()).To(Succeed())
		})
	})
})

var _ = AfterSuite(func() {
	// Cleanup shared resources after all tests have run.
	Expect(os.RemoveAll(tempDir)).To(Succeed())
})
