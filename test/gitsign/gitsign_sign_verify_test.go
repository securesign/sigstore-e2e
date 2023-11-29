package gitsign

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"os"
	"sigstore-e2e-test/pkg/api"
	"sigstore-e2e-test/pkg/clients"
	"sigstore-e2e-test/test/testSupport"
	"time"
)

var _ = Describe("Signing and verifying commits by using Gitsign from the command-line interface", Ordered, func() {
	var gitsign = clients.NewGitsign(testSupport.TestContext)
	var cosign = clients.NewCosign(testSupport.TestContext)

	var (
		dir    string
		config *config.Config
		repo   *git.Repository
		err    error
	)
	BeforeAll(func() {
		Expect(testSupport.InstallPrerequisites(
			gitsign,
			cosign,
		)).To(Succeed())

		DeferCleanup(func() { testSupport.DestroyPrerequisites() })

		// initialize local git repository
		dir, err = os.MkdirTemp("", "repository")
		Expect(err).To(BeNil())
		repo, err = git.PlainInit(dir, false)
		Expect(err).To(BeNil())
		config, err = repo.Config()
		Expect(err).To(BeNil())
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
			config.Raw.AddOption("gitsign", "", "fulcio", api.Values.GetString(api.FulcioURL))
			config.Raw.AddOption("gitsign", "", "rekor", api.Values.GetString(api.RekorURL))
			config.Raw.AddOption("gitsign", "", "issuer", api.Values.GetString(api.OidcIssuerURL))

			Expect(repo.SetConfig(config)).To(Succeed())
		})
	})

	Describe("Make a commit to the local repository", func() {
		It("creates a new file and stage it", func() {
			testFileName := dir + "/testFile.txt"
			Expect(os.WriteFile(testFileName, []byte(uuid.New().String()), 0644)).To(Succeed())
			worktree, err := repo.Worktree()
			Expect(err).To(BeNil())
			_, err = worktree.Add(".")
			Expect(err).To(BeNil())
		})

		It("gets ID token and makes commit", func() {
			token, err := testSupport.GetOIDCToken(api.Values.GetString(api.OidcIssuerURL),
				"jdoe@redhat.com",
				"secure",
				api.Values.GetString(api.OidcRealm))
			Expect(err).To(BeNil())
			Expect(token).To(Not(BeEmpty()))
			Expect(gitsign.GitWithGitSign(dir, token, "commit", "-S", "-m", "CI commit "+time.Now().String())).To(Succeed())
		})

		It("checks that commit has PGP signature", func() {
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

	Describe("Verify the commit", func() {
		Context("With initialized Fulcio CA", func() {
			It("initialize cosign", func() {
				Expect(cosign.Command("initialize",
					"--mirror="+api.Values.GetString(api.TufURL),
					"--root="+api.Values.GetString(api.TufURL)+"/root.json").Run()).To(Succeed())
			})
		})

		When("commiter is authorized", func() {
			It("should verify HEAD signature by gitsign", func() {
				cmd := gitsign.Command("verify",
					"--certificate-identity", "jdoe@redhat.com",
					"--certificate-oidc-issuer", api.Values.GetString(api.OidcIssuerURL),
					"HEAD")
				cmd.Dir = dir
				// gitsign requires to find git in PATH
				cmd.Env = os.Environ()
				Expect(cmd.Run()).To(Succeed())
			})
		})
	})
})
