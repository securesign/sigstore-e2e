package gitsign

import (
	"os"
	"sigstore-e2e-test/pkg/api"
	"sigstore-e2e-test/pkg/clients"
	"sigstore-e2e-test/test/testsupport"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Signing and verifying commits by using Gitsign from the command-line interface", Ordered, func() {
	var gitsign = clients.NewGitsign()
	var cosign = clients.NewCosign()

	var (
		dir    string
		config *config.Config
		repo   *git.Repository
		err    error
	)
	BeforeAll(func() {
		err = testsupport.CheckApiConfigValues(testsupport.Mandatory, api.FulcioURL, api.RekorURL, api.OidcIssuerURL, api.TufURL)
		if err != nil {
			Skip("Skip this test - " + err.Error())
		}

		Expect(testsupport.InstallPrerequisites(
			gitsign,
			cosign,
		)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

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
				Expect(cosign.Command(testsupport.TestContext, "initialize",
					"--mirror="+api.GetValueFor(api.TufURL),
					"--root="+api.GetValueFor(api.TufURL)+"/root.json").Run()).To(Succeed())
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
				Expect(cmd.Run()).To(Succeed())
			})
		})
	})
})
