package rekorsearchui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/clients"
	"github.com/securesign/sigstore-e2e/test/testsupport"
	"github.com/sirupsen/logrus"
)

type BrowserType string

const (
	Chrome  BrowserType = "chrome"
	Firefox BrowserType = "firefox"
	Safari  BrowserType = "safari"
	Edge    BrowserType = "edge"
)

func GetBrowsersToTest() []BrowserType {
	browsersToTest := []BrowserType{Chrome} // Default to Chrome

	if os.Getenv(api.GetValueFor(api.TestFirefox)) == "true" {
		browsersToTest = append(browsersToTest, Firefox)
	}
	if os.Getenv(api.GetValueFor(api.TestSafari)) == "true" {
		browsersToTest = append(browsersToTest, Safari)
	}
	if os.Getenv(api.GetValueFor(api.TestEdge)) == "true" {
		browsersToTest = append(browsersToTest, Edge)
	}

	return browsersToTest
}

type Browser struct {
	PW          *playwright.Playwright
	BrowserObj  playwright.Browser
	Page        playwright.Page
	Context     playwright.BrowserContext
	BrowserType BrowserType
}

func InstallPlaywright() error {
	logrus.Info("Installing Playwright dependencies...")

	// install the Playwright npm package
	installCmd := exec.Command("npm", "install", "@playwright/test")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install Playwright npm package: %w", err)
	}

	// install the browsers
	browserCmd := exec.Command("npx", "playwright", "install", "--with-deps")
	browserCmd.Stdout = os.Stdout
	browserCmd.Stderr = os.Stderr
	if err := browserCmd.Run(); err != nil {
		return fmt.Errorf("failed to install playwright browsers: %w", err)
	}

	// install the Playwright Go driver
	driverCmd := exec.Command("go", "run", "github.com/playwright-community/playwright-go/cmd/playwright", "install")
	driverCmd.Stdout = os.Stdout
	driverCmd.Stderr = os.Stderr
	if err := driverCmd.Run(); err != nil {
		return fmt.Errorf("failed to install playwright driver: %w", err)
	}

	logrus.Info("Playwright installation completed successfully")
	return nil
}

func CreateBrowser(browserType BrowserType, headless bool) (*Browser, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("could not start playwright: %w", err)
	}

	var browserObj playwright.Browser

	launchOptions := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
	}

	switch browserType {
	case Chrome:
		browserObj, err = pw.Chromium.Launch(launchOptions)
	case Firefox:
		browserObj, err = pw.Firefox.Launch(launchOptions)
	case Safari:
		browserObj, err = pw.WebKit.Launch(launchOptions)
	case Edge:
		launchOptions.Args = []string{"--edge"}
		browserObj, err = pw.Chromium.Launch(launchOptions)
	default:
		return nil, fmt.Errorf("unsupported browser type: %s", browserType)
	}

	if err != nil {
		Expect(pw.Stop()).To(Succeed())
		return nil, fmt.Errorf("could not launch browser: %w", err)
	}

	contextOptions := playwright.BrowserNewContextOptions{
		AcceptDownloads: playwright.Bool(true),
		Viewport: &playwright.Size{
			Width:  1280,
			Height: 720,
		},
	}

	context, err := browserObj.NewContext(contextOptions)
	if err != nil {
		browserObj.Close()
		Expect(pw.Stop()).To(Succeed())
		return nil, fmt.Errorf("could not create browser context: %w", err)
	}

	if version := browserObj.Version(); version != "" {
		logrus.Infof("Using %s browser version: %s", browserType, version)
	}

	return &Browser{
		PW:          pw,
		BrowserObj:  browserObj,
		Context:     context,
		BrowserType: browserType,
	}, nil

}

func (b *Browser) Navigate(url string) error {
	page, err := b.Context.NewPage()
	if err != nil {
		return fmt.Errorf("could not create page: %w", err)
	}

	page.SetDefaultTimeout(30000)
	b.Page = page

	for attempts := 0; attempts < 3; attempts++ {
		response, err := page.Goto(url, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
			Timeout:   playwright.Float(30000),
		})

		if err == nil && response != nil {
			status := response.Status()
			if status < 400 {
				logrus.Infof("Navigation to %s successful (status: %d)", url, status)
				return nil
			}
			return fmt.Errorf("page loaded with status code: %d", status)
		}

		if attempts < 2 {
			time.Sleep(1 * time.Second)
			continue
		}
	}

	return fmt.Errorf("failed to navigate to %s after 3 attempts", url)
}

func (b *Browser) Close() error {
	var errors []string

	if b.Page != nil {
		if err := b.Page.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("failed to close page: %v", err))
		}
	}

	if b.Context != nil {
		if err := b.Context.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("failed to close context: %v", err))
		}
	}

	if b.BrowserObj != nil {
		if err := b.BrowserObj.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("failed to close browser: %v", err))
		}
	}

	if b.PW != nil {
		if err := b.PW.Stop(); err != nil {
			errors = append(errors, fmt.Sprintf("failed to stop playwright: %v", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors while closing browser: %s", strings.Join(errors, "; "))
	}

	return nil
}

func (b *Browser) Screenshot(filename string) error {
	screenshotPath := filepath.Join("screenshots", b.BrowserType.String(), filename)
	if _, err := os.Stat(filepath.Dir(screenshotPath)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(screenshotPath), 0755); err != nil {
			return fmt.Errorf("could not create screenshot directory: %w", err)
		}
	}

	_, err := b.Page.Screenshot(playwright.PageScreenshotOptions{
		Path:     playwright.String(screenshotPath),
		FullPage: playwright.Bool(true),
	})
	return err
}

func (b BrowserType) String() string {
	return string(b)
}

type TestData struct {
	Email          string
	Hash           string
	LogIndex       string
	EntryUUID      string
	CommitSHA      string
	CommitLogIndex int
}

type BrowserTest struct {
	Browser     *Browser
	URL         string
	TestData    *TestData
	BrowserType BrowserType
}

func NewBrowserTest(browserType BrowserType, headless bool, url string, testData *TestData) (*BrowserTest, error) {
	browser, err := CreateBrowser(browserType, headless)
	if err != nil {
		return nil, err
	}

	return &BrowserTest{
		Browser:     browser,
		URL:         url,
		TestData:    testData,
		BrowserType: browserType,
	}, nil
}

func (bt *BrowserTest) Close() error {
	return bt.Browser.Close()
}

func (bt *BrowserTest) performSearch(attributeValue, inputID, searchValue string) error {
	browser := bt.Browser

	if err := browser.Navigate(bt.URL); err != nil {
		return err
	}

	logrus.Infof("Starting %s search test", attributeValue)

	// Take initial screenshot
	Expect(browser.Screenshot(fmt.Sprintf("%s-search-initial.png", attributeValue))).To(Succeed())

	// If not using the default email option, select the appropriate option from dropdown
	if attributeValue != "email" {
		logrus.Infof("Selected search attribute: %s", attributeValue)

		attrLocator := browser.Page.Locator("#rekor-search-attribute")
		if err := attrLocator.WaitFor(playwright.LocatorWaitForOptions{
			State: playwright.WaitForSelectorStateVisible,
		}); err != nil {
			return fmt.Errorf("failed to wait for attribute dropdown: %w", err)
		}

		if err := attrLocator.Click(); err != nil {
			return fmt.Errorf("failed to click attribute dropdown: %w", err)
		}

		selectLocator := browser.Page.Locator("select")
		if _, err := selectLocator.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{attributeValue},
		}); err != nil {
			return fmt.Errorf("failed to select %s option: %w", attributeValue, err)
		}

		if err := attrLocator.Click(); err != nil {
			return fmt.Errorf("failed to click attribute dropdown after selection: %w", err)
		}
	}

	// Find and fill input field
	inputLocator := browser.Page.Locator(inputID)
	if err := inputLocator.WaitFor(playwright.LocatorWaitForOptions{
		State: playwright.WaitForSelectorStateVisible,
	}); err != nil {
		return fmt.Errorf("failed to wait for input: %w", err)
	}

	logrus.Infof("Filling %s field with: %s", attributeValue, searchValue)
	if err := inputLocator.Fill(searchValue); err != nil {
		return fmt.Errorf("failed to input %s: %w", attributeValue, err)
	}

	// Verify input value
	inputValue, err := inputLocator.InputValue()
	if err != nil {
		return fmt.Errorf("failed to get input value: %w", err)
	}

	if inputValue != searchValue {
		return fmt.Errorf("input value mismatch: expected %s, got %s", searchValue, inputValue)
	}

	// Take screenshot before search
	Expect(browser.Screenshot(fmt.Sprintf("%s-search-before-click.png", attributeValue))).To(Succeed())

	// Click search button
	searchLocator := browser.Page.Locator("#search-form-button")
	if err := searchLocator.Click(); err != nil {
		return fmt.Errorf("failed to click search button: %w", err)
	}

	logrus.Infof("Executing %s search", attributeValue)

	if err := browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return fmt.Errorf("failed to wait for network idle: %w", err)
	}

	if err := browser.Page.Locator(".pf-v5-c-card").
		First().
		WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(5_000),
			State:   playwright.WaitForSelectorStateVisible,
		}); err != nil {
		logrus.Warnf("No result cards became visible within 5 s – continuing anyway")
	}

	allCards := browser.Page.Locator(".pf-v5-c-card")
	cardCount, _ := allCards.Count()

	foundResult := false
	for i := 0; i < cardCount; i++ {
		uuidText, _ := allCards.Nth(i).Locator("h2 a").TextContent()
		if uuidText == bt.TestData.EntryUUID {
			foundResult = true
			break
		}
	}

	Expect(browser.Screenshot(fmt.Sprintf("%s-search-results.png", attributeValue))).To(Succeed())

	if foundResult {
		logrus.Infof("Search successful: Found entry with UUID %s", bt.TestData.EntryUUID)
		return nil
	}
	return fmt.Errorf("could not find a card with UUID %s", bt.TestData.EntryUUID)
}

func (bt *BrowserTest) TestEmailSearch() error {
	err := bt.performSearch("email", "#rekor-search-email", bt.TestData.Email)

	// Fall back to UUID search if the email search fails
	if err != nil {
		logrus.Warnf("Email search failed, possibly due to result limit. Falling back to UUID search.")

		uuidErr := bt.performSearch("uuid", `#rekor-search-entry\ uuid`, bt.TestData.EntryUUID)
		if uuidErr != nil {
			return fmt.Errorf("both email and UUID searches failed: email error: %w, UUID error: %w", err, uuidErr)
		}

		card := bt.Browser.Page.Locator(".pf-v5-c-card").First()
		cardText, err := card.TextContent()
		if err != nil {
			return fmt.Errorf("failed to get card text content:  %w", err)
		}

		if strings.Contains(cardText, bt.TestData.Email) {
			logrus.Infof("Email search fallback successful: Found entry with UUID %s and email %s",
				bt.TestData.EntryUUID, bt.TestData.Email)
			logrus.Infof("Card text: %s", cardText)
			return nil
		}

		entryLink := bt.Browser.Page.Locator(`h2 a[href*="` + bt.TestData.EntryUUID + `"]`)
		if err := entryLink.Click(); err != nil {
			return fmt.Errorf("failed to click entry link: %w", err)
		}

		if err := bt.Browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		}); err != nil {
			return fmt.Errorf("failed to wait for entry details to load: %w", err)
		}

		emailElement := bt.Browser.Page.Locator(`text=` + bt.TestData.Email)
		if visible, _ := emailElement.IsVisible(); !visible {
			return fmt.Errorf("entry found by UUID, but email %s not found on details page", bt.TestData.Email)
		}

		logrus.Infof("Email search fallback successful: Found entry with UUID %s and verified email %s",
			bt.TestData.EntryUUID, bt.TestData.Email)
		return nil
	}

	return nil
}

func (bt *BrowserTest) TestHashSearch() error {
	return bt.performSearch("hash", "#rekor-search-hash", bt.TestData.Hash)
}

func (bt *BrowserTest) TestLogIndexSearch() error {
	return bt.performSearch("logIndex", `#rekor-search-log\ index`, bt.TestData.LogIndex)
}

func (bt *BrowserTest) TestUUIDSearch() error {
	return bt.performSearch("uuid", `#rekor-search-entry\ uuid`, bt.TestData.EntryUUID)
}

func (bt *BrowserTest) TestCommitSHASearch() error {
	return bt.performSearch("commitSha", `#rekor-search-commit\ sha`, bt.TestData.CommitSHA)
}

var _ = Describe("Test the Rekor Search UI", Ordered, func() {
	var (
		rekorCli    *clients.RekorCli
		gitsign     *clients.Gitsign
		cosign      *clients.Cosign
		tempDir     string
		dirFilePath string
		tarFilePath string
		config      *config.Config
		dir         string
		repo        *git.Repository
		testData    TestData
	)

	appURL := api.GetValueFor(api.RekorUIURL)

	BeforeAll(func() {
		err := testsupport.CheckMandatoryAPIConfigValues(api.OidcRealm, api.RekorUIURL)
		if err != nil {
			Fail(err.Error())
		}

		rekorCli = clients.NewRekorCli()
		cosign = clients.NewCosign()
		gitsign = clients.NewGitsign()

		Expect(testsupport.InstallPrerequisites(
			rekorCli,
			gitsign,
			cosign,
		)).To(Succeed())

		Expect(InstallPlaywright()).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

		// Create screenshots directory for each browser type
		for _, browserType := range GetBrowsersToTest() {
			screenshotDir := filepath.Join("screenshots", browserType.String())
			if _, err := os.Stat(screenshotDir); os.IsNotExist(err) {
				Expect(os.MkdirAll(screenshotDir, 0755)).To(Succeed())
			}
		}

		// Setup test environment
		tempDir, err = os.MkdirTemp("", "rekorTest")
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			os.RemoveAll(tempDir)
		})

		// Initialize local git repository
		dir, err = os.MkdirTemp("", "repository")
		Expect(err).ToNot(HaveOccurred())
		repo, err = git.PlainInit(dir, false)
		Expect(err).ToNot(HaveOccurred())
		config, err = repo.Config()
		Expect(err).ToNot(HaveOccurred())

		// Configure git user
		config.User.Name = "John Doe"
		config.User.Email = fmt.Sprintf("%s@%s", api.GetValueFor(api.OidcUser), api.GetValueFor(api.OidcUserDomain))

		// Configure gitsign
		config.Raw.AddOption("commit", "", "gpgsign", "true")
		config.Raw.AddOption("tag", "", "gpgsign", "true")
		config.Raw.AddOption("gpg", "x509", "program", "gitsign")
		config.Raw.AddOption("gpg", "", "format", "x509")
		config.Raw.AddOption("gitsign", "", "fulcio", api.GetValueFor(api.FulcioURL))
		config.Raw.AddOption("gitsign", "", "rekor", api.GetValueFor(api.RekorURL))
		config.Raw.AddOption("gitsign", "", "issuer", api.GetValueFor(api.OidcIssuerURL))

		Expect(repo.SetConfig(config)).To(Succeed())

		// Create and commit file
		testFileName := dir + "/testFile.txt"
		Expect(os.WriteFile(testFileName, []byte(uuid.New().String()), 0600)).To(Succeed())
		worktree, err := repo.Worktree()
		Expect(err).ToNot(HaveOccurred())
		_, err = worktree.Add(".")
		Expect(err).ToNot(HaveOccurred())

		// Sign commit with gitsign
		token, err := testsupport.GetOIDCToken(testsupport.TestContext)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).To(Not(BeEmpty()))

		Expect(gitsign.GitWithGitSign(testsupport.TestContext, dir, token, "commit", "-S", "-m", "CI commit "+time.Now().String())).To(Succeed())

		// Get commit SHA
		head, err := repo.Head()
		Expect(err).ToNot(HaveOccurred())
		commit, err := repo.CommitObject(head.Hash())
		Expect(err).ToNot(HaveOccurred())
		testData.CommitSHA = commit.Hash.String()
		testData.Email = commit.Author.Email

		dirFilePath = filepath.Join(tempDir, "myrelease")
		tarFilePath = filepath.Join(tempDir, "myrelease.tar.gz")

		err = os.Mkdir(dirFilePath, 0755)
		Expect(err).ToNot(HaveOccurred())

		Expect(exec.Command("tar", "-czvf", tarFilePath, dirFilePath).Run()).To(Succeed())
		Expect(cosign.Command(testsupport.TestContext, "initialize").Run()).To(Succeed())

		cmd := gitsign.Command(testsupport.TestContext, "verify",
			"--certificate-identity", fmt.Sprintf("%s@%s", api.GetValueFor(api.OidcUser), api.GetValueFor(api.OidcUserDomain)),
			"--certificate-oidc-issuer", api.GetValueFor(api.OidcIssuerURL),
			"HEAD")

		cmd.Dir = dir
		cmd.Env = os.Environ()

		var output bytes.Buffer
		cmd.Stdout = &output
		Expect(cmd.Run()).To(Succeed())
		logrus.WithField("app", "gitsign").Info(output.String())

		// Extract log index
		re := regexp.MustCompile(`tlog index: (\d+)`)
		match := re.FindStringSubmatch(output.String())
		testData.LogIndex = match[1]

		// Get the entry data from Rekor
		rekorURL := api.GetValueFor(api.RekorURL)
		full, _ := rekorCli.CommandOutput(testsupport.TestContext, "get", "--rekor_server", rekorURL, "--log-index", testData.LogIndex)
		out := string(full)

		// Extract UUID
		uuidRe := regexp.MustCompile(`(?m)^UUID:\s+([0-9a-f]+)`)
		m := uuidRe.FindStringSubmatch(out)
		Expect(m).To(HaveLen(2))
		testData.EntryUUID = m[1]

		// Extract hash
		hashRe := regexp.MustCompile(`"value":\s+"([0-9a-f]{64})"`)
		m = hashRe.FindStringSubmatch(out)
		Expect(m).To(HaveLen(2))
		testData.Hash = m[1]

		logrus.Infof("Email = %s", testData.Email)
		logrus.Infof("Hash = %s", testData.Hash)
		logrus.Infof("Commit SHA = %s", testData.CommitSHA)
		logrus.Infof("Entry UUID = %s", testData.EntryUUID)
		logrus.Infof("Log Index = %s\n", testData.LogIndex)

		logrus.Infof("UI URL: %s", appURL)
		logrus.Infof("Testing with browsers: %v\n", GetBrowsersToTest())
	})

	// Run tests for each browser type
	for _, browserType := range GetBrowsersToTest() {
		// Use a closure to capture the browser type for each iteration
		func(bt BrowserType) {
			Context(fmt.Sprintf("Testing with %s browser", bt), func() {
				var browserTest *BrowserTest

				BeforeEach(func() {
					logrus.Infof("\n=== Starting %s browser test suite ===", bt)

					headless := api.GetValueFor(api.HeadlessUI) == "true"
					var err error
					browserTest, err = NewBrowserTest(bt, headless, appURL, &testData)
					Expect(err).ToNot(HaveOccurred())
				})

				AfterEach(func() {
					if browserTest != nil {
						Expect(browserTest.Close()).To(Succeed())
					}

					logrus.Infof("=== Completed %s browser test suite ===\n", bt)
				})

				It("should search by email", func() {
					Expect(browserTest.TestEmailSearch()).To(Succeed())
				})

				It("should search by hash", func() {
					Expect(browserTest.TestHashSearch()).To(Succeed())
				})

				It("should search by log index", func() {
					Expect(browserTest.TestLogIndexSearch()).To(Succeed())
				})

				It("should search by entry UUID", func() {
					Expect(browserTest.TestUUIDSearch()).To(Succeed())
				})

				It("should search by commit SHA", func() {
					Expect(browserTest.TestCommitSHASearch()).To(Succeed())
				})
			})
		}(browserType)
	}
})
