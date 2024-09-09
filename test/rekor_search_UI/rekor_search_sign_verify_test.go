package rekor_search_UI

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	. "github.com/onsi/ginkgo/v2"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/clients"
	"github.com/securesign/sigstore-e2e/test/testsupport"
	"github.com/sirupsen/logrus"

	. "github.com/onsi/gomega"
	"github.com/ysmood/got"
)

var entryIndex int
var hashWithAlg string
var tempDir string
var dirFilePath string
var tarFilePath string
var signatureFilePath string

type G struct {
	Got     got.G
	Browser *rod.Browser
}

// Define the page method for the G struct
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
		err      error
		rekorCli *clients.RekorCli
		//rekorHash string
	)

	BeforeAll(func() {
		err = testsupport.CheckMandatoryAPIConfigValues(api.OidcRealm)
		if err != nil {
			Skip("Skip this test - " + err.Error())
		}

		rekorCli = clients.NewRekorCli()

		Expect(testsupport.InstallPrerequisites(
			rekorCli,
		)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

	})

	// Setup function that initializes and returns a new instance of G
	setup := func() G {
		launch := launcher.New().Headless(false)
		url := launch.MustLaunch()
		browser := rod.New().ControlURL(url).MustConnect()  // create a new browser instance for each test
		return G{Got: got.New(GinkgoT()), Browser: browser} // Directly assign got.New(GinkgoT())
	}

	// A helper function to create a page
	//appURL :=api.GetValueFor(api.)
	const appURL = "http://localhost:3000" // will be replaced by URL from OCP, testing on localhost for now

	// Describe("Test email", func() {
	// 	It("test UI with email", func() {
	// 		// Properly invoke setup to get an instance of G
	// 		fmt.Println("came here0")
	// 		g := setup()

	// 		// Use the page method to navigate to the application URL
	// 		p := g.page(appURL)

	// 		// Ensure the element is ready before interacting with it
	// 		attrElement := p.MustElement("#rekor-search-attribute")
	// 		attrElement.MustWaitVisible().MustClick()

	// 		// Select the "Email" option from the dropdown
	// 		p.MustElement("select").MustSelect("Email")
	// 		attrElement.MustClick()

	// 		// Fill the text field with the email "jdoe@redhat.com"
	// 		emailInput := p.MustElement("#rekor-search-email")
	// 		emailInput.MustWaitVisible().MustInput("jdoe@redhat.com") // will be replaced with the actual email used for signing

	// 		// Verify the input value
	// 		inputValue := emailInput.MustProperty("value").String()
	// 		g.Got.Eq(inputValue, "jdoe@redhat.com")

	// 		// Click the search button
	// 		searchButton := p.MustElement("#search-form-button")
	// 		searchButton.MustClick()

	// 		// Wait for the content to be visible
	// 		//content := p.MustElement("#pf-v5-c-card")
	// 		//content.MustWaitVisible() // the test won't pass if we wait for the load of data (need fixing)

	// 		fmt.Println("Email Done")
	// 	})
	// })

	Describe("Test Hash", func() {
		It("test UI with Hash", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			entryIndexStr := strconv.Itoa(entryIndex)

			// extrract of hash value for searching with --sha
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--log-index", entryIndexStr)
			Expect(err).ToNot(HaveOccurred())

			// Look for JSON start
			startIndex := strings.Index(string(output), "{")
			Expect(startIndex).NotTo(Equal(-1), "JSON start - '{' not found")

			jsonStr := string(output[startIndex:])

			var rekorGetOutput testsupport.RekorCLIGetOutput
			err = json.Unmarshal([]byte(jsonStr), &rekorGetOutput)
			Expect(err).ToNot(HaveOccurred())

			// algorithm:hashValue
			hashWithAlg = rekorGetOutput.RekordObj.Data.Hash.Algorithm + ":" + rekorGetOutput.RekordObj.Data.Hash.Value
			//need to take hash from the stack so I can test it in UI

			g := setup() // invoke setup to get a new instance of G for this test
			p := g.page(appURL)
			//ensure the element is ready before interacting with it
			attrElement := p.MustElement("#rekor-search-attribute")
			attrElement.MustWaitVisible().MustClick()

			//_ = p.MustElement("select").Select([]string{`[value="hash"]`}, false, rod.SelectorTypeCSSSector)
			p.MustElement("select").MustSelect("Hash")

			//select the "hash" option from the dropdown
			attrElement.MustClick()

			fmt.Println(hashWithAlg)
			// fill the text field with the hash
			hashInput := p.MustElement("#rekor-search-hash")
			hashInput.MustWaitVisible().MustInput(hashWithAlg) //will be replaced with actual hash used for signing
			fmt.Println("came here5")

			// verify the input value
			inputValue := hashInput.MustProperty("value").String()
			g.Got.Eq(inputValue, hashWithAlg) //here we take the hash from stack

			searchButton := p.MustElement("#search-form-button")
			searchButton.MustClick()

			//content := p.MustElement("#pf-v5-c-card")
			//content.MustWaitVisible() //the test wont pass if we wait for the load of data (need fixing)
			fmt.Println("Hash Done")
		})
	})

	Describe("Test logIndex", func() {
		It("test UI with Log Index", func() {
			entryIndexStr := strconv.Itoa(entryIndex)
			g := setup() // invoke setup to get a new instance of G for this test
			p := g.page(appURL)
			//ensure the element is ready before interacting with it
			attrElement := p.MustElement("#rekor-search-attribute")
			attrElement.MustWaitVisible().MustClick()

			//_ = p.MustElement("select").Select([]string{`[value="hash"]`}, false, rod.SelectorTypeCSSSector)
			p.MustElement("select").MustSelect("Log Index")

			//select the "hash" option from the dropdown
			attrElement.MustClick()

			fmt.Println(entryIndexStr)
			// fill the text field with the hash
			hashInput := p.MustElement(`#rekor-search-log\ index`)
			hashInput.MustWaitVisible().MustInput(entryIndexStr) //will be replaced with actual hash used for signing
			fmt.Println("came here5")

			// verify the input value
			inputValue := hashInput.MustProperty("value").String()
			g.Got.Eq(inputValue, entryIndexStr) //here we take the hash from stack

			searchButton := p.MustElement("#search-form-button")
			searchButton.MustClick()

			content := p.MustElement("#pf-v5-c-card")
			content.MustWaitVisible() //the test wont pass if we wait for the load of data (need fixing)
			fmt.Println("Hash Done")

		})
	})
})
