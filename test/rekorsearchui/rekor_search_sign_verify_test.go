package rekorsearchui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/clients"
	"github.com/securesign/sigstore-e2e/test/testsupport"
	"github.com/sirupsen/logrus"
	"github.com/ysmood/got"
)

var browser *rod.Browser
var page *rod.Page

type G struct {
	Got     got.G
	Browser *rod.Browser
}

func (g G) page(url string) *rod.Page {
	page := g.Browser.MustPage(url)
	DeferCleanup(func() {
		if err := page.Close(); err != nil {
			g.Got.Logf("Failed to close page: %v", err)
		}
	})
	return page
}

var _ = Describe("Test the Rekor Search UI", Ordered, func() {
	var (
		err               error
		rekorCli          *clients.RekorCli
		gitsign           *clients.Gitsign
		tempDir           string
		dirFilePath       string
		tarFilePath       string
		config            *config.Config
		signatureFilePath string
		dir               string
		repo              *git.Repository
		testData          struct {
			Email          string
			Hash           string
			LogIndex       int
			EntryUUID      string
			CommitSHA      string
			CommitLogIndex int
		}
	)

	appURL := api.GetValueFor(api.RekorUIURL)

	setup := func() G {
		headless := api.GetValueFor(api.HeadlessUI) == "true"
		launch := launcher.New().Headless(headless)
		url := launch.MustLaunch()
		browser := rod.New().ControlURL(url).MustConnect()

		g := G{Got: got.New(GinkgoT()), Browser: browser}
		DeferCleanup(func() {
			if err := browser.Close(); err != nil {
				g.Got.Logf("Failed to close browser: %v", err)
			}
		})

		return g
	}

	BeforeAll(func() {
		err = testsupport.CheckMandatoryAPIConfigValues(api.OidcRealm)
		err = testsupport.CheckMandatoryAPIConfigValues(api.RekorUIURL)
		if err != nil {
			Skip("Skip this test - " + err.Error())
		}

		rekorCli = clients.NewRekorCli()
		gitsign = clients.NewGitsign()

		Expect(testsupport.InstallPrerequisites(
			rekorCli,
			gitsign,
		)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

		tempDir, err = os.MkdirTemp("", "rekorTest")
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			os.RemoveAll(tempDir)
		})
		dir, err = os.MkdirTemp("", "repository")
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			os.RemoveAll(dir)
		})
		repo, err = git.PlainInit(dir, false)
		Expect(err).ToNot(HaveOccurred())
		config, err = repo.Config()
		Expect(err).ToNot(HaveOccurred())

		// configure git user
		config.User.Name = "John Doe"
		config.User.Email = "jdoe@redhat.com"

		// configure gitsign
		config.Raw.AddOption("commit", "", "gpgsign", "true")
		config.Raw.AddOption("tag", "", "gpgsign", "true")
		config.Raw.AddOption("gpg", "x509", "program", "gitsign")
		config.Raw.AddOption("gpg", "", "format", "x509")
		config.Raw.AddOption("gitsign", "", "fulcio", api.GetValueFor(api.FulcioURL))
		config.Raw.AddOption("gitsign", "", "rekor", api.GetValueFor(api.RekorURL))
		config.Raw.AddOption("gitsign", "", "issuer", api.GetValueFor(api.OidcIssuerURL))

		Expect(repo.SetConfig(config)).To(Succeed())

		// create and commit file
		testFileName := filepath.Join(dir, "testFile.txt")
		Expect(os.WriteFile(testFileName, []byte(uuid.New().String()), 0600)).To(Succeed())

		worktree, err := repo.Worktree()
		Expect(err).ToNot(HaveOccurred())

		_, err = worktree.Add(".")
		Expect(err).ToNot(HaveOccurred())

		// sign commit with gitsign
		token, err := testsupport.GetOIDCToken(testsupport.TestContext,
			api.GetValueFor(api.OidcIssuerURL), "jdoe@redhat.com", "secure", api.GetValueFor(api.OidcRealm))
		Expect(err).ToNot(HaveOccurred())
		Expect(token).To(Not(BeEmpty()))

		Expect(gitsign.GitWithGitSign(testsupport.TestContext, dir, token, "commit", "-S", "-m", "CI commit "+time.Now().String())).To(Succeed())
		rekorServerURL := api.GetValueFor(api.RekorURL)

		// get commit SHA
		head, err := repo.Head()
		Expect(err).ToNot(HaveOccurred())
		commit, err := repo.CommitObject(head.Hash())
		Expect(err).ToNot(HaveOccurred())
		testData.CommitSHA = commit.Hash.String()
		testData.Email = commit.Author.Email

		dirFilePath = filepath.Join(tempDir, "myrelease")
		tarFilePath = filepath.Join(tempDir, "myrelease.tar.gz")
		signatureFilePath = filepath.Join(tempDir, "mysignature.asc")

		err = os.Mkdir(dirFilePath, 0755)
		Expect(err).ToNot(HaveOccurred())

		// taring artifact
		tarCmd := exec.Command("tar", "-czvf", tarFilePath, dirFilePath)
		err = tarCmd.Run()
		Expect(err).ToNot(HaveOccurred())

		// signing artifact
		signCmd := exec.Command("openssl", "dgst", "-sha256", "-sign", "private.pem", "-out", signatureFilePath, tarFilePath)

		err = signCmd.Run()
		Expect(err).ToNot(HaveOccurred())

		// uploading data to rekor
		output, err := rekorCli.CommandOutput(testsupport.TestContext, "upload", "--rekor_server", rekorServerURL, "--artifact", tarFilePath, "--signature", signatureFilePath, "--pki-format=x509", "--public-key", "public.pem")
		Expect(err).ToNot(HaveOccurred())

		// extract logindex
		outputStr := string(output)
		logIndexStart := strings.Index(outputStr, "LogIndex: ")
		if logIndexStart != -1 {
			logIndexEnd := strings.IndexAny(outputStr[logIndexStart:], "\n")
			testData.LogIndex, _ = strconv.Atoi(strings.TrimSpace(outputStr[logIndexStart+len("LogIndex: ") : logIndexStart+logIndexEnd]))
		}

		entryIndexStr := strconv.Itoa(testData.LogIndex)
		fullOutput, err := rekorCli.CommandOutput(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--log-index", entryIndexStr)
		Expect(err).ToNot(HaveOccurred())

		// extract UUID
		uuidStart := strings.Index(string(fullOutput), "UUID: ")
		if uuidStart != -1 {
			uuidEnd := strings.IndexAny(string(fullOutput)[uuidStart+len("UUID: "):], " \n")
			if uuidEnd != -1 {
				testData.EntryUUID = string(fullOutput)[uuidStart+len("UUID: ") : uuidStart+len("UUID: ")+uuidEnd]
			}
		}

		startIndex := strings.Index(string(fullOutput), "{")
		Expect(startIndex).NotTo(Equal(-1), "JSON start - '{' not found")

		var rekorGetOutput testsupport.RekorCLIGetOutput
		err = json.Unmarshal(fullOutput[startIndex:], &rekorGetOutput)

		Expect(err).ToNot(HaveOccurred())
		// algorithm with hash
		testData.Hash = rekorGetOutput.RekordObj.Data.Hash.Algorithm + ":" + rekorGetOutput.RekordObj.Data.Hash.Value
		testData.Email = commit.Author.Email
	})

	Describe("Test email search", func() {
		It("should search by email in Rekor UI", func() {
			g := setup()
			p := g.page(appURL)
			defer p.Close()
			defer g.Browser.Close()

			attrElement := p.MustElement("#rekor-search-attribute")
			attrElement.MustWaitVisible().MustClick()

			p.MustElement("select").MustSelect("Email")
			attrElement.MustClick()

			emailInput := p.MustElement("#rekor-search-email")
			emailInput.MustWaitVisible().MustInput(testData.Email)

			inputValue := emailInput.MustProperty("value").String()
			g.Got.Eq(inputValue, testData.Email)

			searchButton := p.MustElement("#search-form-button")
			searchButton.MustClick()

			content := p.MustElement(".pf-v5-c-card")
			content.MustWaitVisible()
			// verify if the data has been presented right
			codeElement := p.MustElement("code.language-yaml")
			codeText := codeElement.MustText()
			Expect(codeText).To(ContainSubstring("email:"))
			Expect(codeText).To(ContainSubstring(testData.Email))
			fmt.Printf("Search with Email:\n %s\n", codeText)
		})
	})

	Describe("Test Hash search", func() {
		It("should search by Hash in Rekor UI", func() {
			g := setup()
			browser = g.Browser
			p := g.page(appURL)
			page = p
			defer p.Close()

			attrElement := p.MustElement("#rekor-search-attribute")
			attrElement.MustWaitVisible().MustClick()

			p.MustElement("select").MustSelect("Hash")
			attrElement.MustClick()

			hashInput := p.MustElement("#rekor-search-hash")
			hashInput.MustWaitVisible().MustInput(testData.Hash)

			inputValue := hashInput.MustProperty("value").String()
			g.Got.Eq(inputValue, testData.Hash)

			searchButton := p.MustElement("#search-form-button")
			searchButton.MustClick()

			content := p.MustElement(".pf-v5-c-card")
			content.MustWaitVisible()

			codeElement := p.MustElement("code.language-yaml")
			codeText := codeElement.MustText()
			// verify if the data has been presented right
			hashParts := strings.Split(testData.Hash, ":")
			Expect(codeText).To(ContainSubstring(fmt.Sprintf("algorithm: %s", hashParts[0])))
			Expect(codeText).To(ContainSubstring(fmt.Sprintf("value: %s", hashParts[1])))
			fmt.Printf("Search with hash:\n %s\n", codeText)
		})
	})

	Describe("Test Log Index search", func() {
		It("should search by Log Index in Rekor UI", func() {
			g := setup()
			browser = g.Browser
			p := g.page(appURL)
			page = p
			defer p.Close()

			attrElement := p.MustElement("#rekor-search-attribute")
			attrElement.MustWaitVisible().MustClick()

			p.MustElement("select").MustSelect("Log Index")
			attrElement.MustClick()

			entryIndexStr := strconv.Itoa(testData.LogIndex)
			logIndexInput := p.MustElement(`#rekor-search-log\ index`)
			logIndexInput.MustWaitVisible().MustInput(entryIndexStr)

			inputValue := logIndexInput.MustProperty("value").String()
			g.Got.Eq(inputValue, entryIndexStr)

			searchButton := p.MustElement("#search-form-button")
			searchButton.MustClick()
			content := p.MustElement(".pf-v5-c-card__body")
			content.MustWaitVisible()
			// verify if the data has been presented right
			codeText := content.MustText()
			Expect(codeText).To(ContainSubstring(fmt.Sprintf("Log Index\n\n%s", entryIndexStr)))
			fmt.Printf("Search with Log Index:\n %s\n", codeText)
		})
	})

	Describe("Test UUID search", func() {
		It("should search by Entry UUID in Rekor UI", func() {
			g := setup()
			browser = g.Browser
			p := g.page(appURL)
			page = p
			defer p.Close()

			attrElement := p.MustElement("#rekor-search-attribute")
			attrElement.MustWaitVisible().MustClick()

			p.MustElement("select").MustSelect("Entry UUID")
			attrElement.MustClick()

			uuidInput := p.MustElement(`#rekor-search-entry\ uuid`)
			uuidInput.MustWaitVisible().MustInput(testData.EntryUUID)

			inputValue := uuidInput.MustProperty("value").String()
			g.Got.Eq(inputValue, testData.EntryUUID)

			searchButton := p.MustElement("#search-form-button")
			searchButton.MustClick()

			content := p.MustElement(".pf-v5-c-card")
			content.MustWaitVisible()
			// verify if the data has been presented right
			codeText := content.MustText()
			Expect(codeText).To(ContainSubstring(fmt.Sprintf("Entry UUID: %s", testData.EntryUUID)))
			fmt.Printf("Search with UUID:\n %s\n", codeText)
		})

	})
	Describe("Test Commit SHA search", func() {
		It("should search by Commit SHA in Rekor UI", func() {
			g := setup()
			browser = g.Browser
			p := g.page(appURL)
			page = p

			attrElement := p.MustElement("#rekor-search-attribute")
			attrElement.MustWaitVisible().MustClick()

			p.MustElement("select").MustSelect("Commit SHA")
			attrElement.MustClick()

			shaInput := p.MustElement(`#rekor-search-commit\ sha`)
			shaInput.MustWaitVisible().MustInput(testData.CommitSHA)

			inputValue := shaInput.MustProperty("value").String()
			g.Got.Eq(inputValue, testData.CommitSHA)

			searchButton := p.MustElement("#search-form-button")
			searchButton.MustClick()

			content := p.MustElement(".pf-v5-c-card")
			content.MustWaitVisible()

			codeElement := p.MustElement("code.language-yaml")
			codeText := codeElement.MustText()
			// here is just checking if commiter email is right
			Expect(codeText).To(ContainSubstring("email:"))
			Expect(codeText).To(ContainSubstring(testData.Email))
			fmt.Printf("Search with SHA commit:\n %s\n", codeText)
		})
	})

	AfterEach(func() {
		if browser != nil {
			_ = browser.Close() // close the browser
			browser = nil
		}

	})

})
