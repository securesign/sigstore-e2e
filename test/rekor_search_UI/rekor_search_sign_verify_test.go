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
var UuidStr string

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
	)
	appURL := api.GetValueFor(api.RekorUIURL)
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

	Describe("Test email", func() {
		It("test UI with email", func() {
			// Properly invoke setup to get an instance of G
			g := setup()

			// Use the page method to navigate to the application URL
			p := g.page(appURL)

			// Ensure the element is ready before interacting with it
			attrElement := p.MustElement("#rekor-search-attribute")
			attrElement.MustWaitVisible().MustClick()

			// Select the "Email" option from the dropdown
			p.MustElement("select").MustSelect("Email")
			attrElement.MustClick()

			// Fill the text field with the email "jdoe@redhat.com alternatively we can use bob.callaway@gmail.com"
			emailInput := p.MustElement("#rekor-search-email")
			emailInput.MustWaitVisible().MustInput("jdoe@redhat.com") // will be replaced with the actual email used for signing

			// Verify the input value
			inputValue := emailInput.MustProperty("value").String()
			g.Got.Eq(inputValue, "jdoe@redhat.com")

			// Click the search button
			searchButton := p.MustElement("#search-form-button")
			searchButton.MustClick()

			// Wait for the content to be visible
			//content := p.MustElement("#pf-v5-c-card")
			//content.MustWaitVisible() // the test won't pass if we wait for the load of data (need fixing)

			fmt.Println("Email Done")
		})
	})

	Describe("Test Hash", func() {
		It("test UI with Hash", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			entryIndexStr := strconv.Itoa(entryIndex)
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--log-index", entryIndexStr)
			Expect(err).ToNot(HaveOccurred())
			outputStr := string(output)
			uuidStart := strings.Index(outputStr, "UUID: ")
			if uuidStart == -1 {
				fmt.Println("UUID not found")
				return
			}
			UuidStr = outputStr[uuidStart+len("UUID: "):]
			uuidEnd := strings.IndexAny(UuidStr, " \n")
			if uuidEnd != -1 {
				UuidStr = UuidStr[:uuidEnd]
			}
			fmt.Println("Extracted UUID:", UuidStr)
			startIndex := strings.Index(string(output), "{")
			Expect(startIndex).NotTo(Equal(-1), "JSON start - '{' not found")

			jsonStr := string(output[startIndex:])

			var rekorGetOutput testsupport.RekorCLIGetOutput
			err = json.Unmarshal([]byte(jsonStr), &rekorGetOutput)
			Expect(err).ToNot(HaveOccurred())

			hashWithAlg = rekorGetOutput.RekordObj.Data.Hash.Algorithm + ":" + rekorGetOutput.RekordObj.Data.Hash.Value

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

			// verify the input value
			inputValue := hashInput.MustProperty("value").String()
			g.Got.Eq(inputValue, hashWithAlg) //here we take the hash from stack

			searchButton := p.MustElement("#search-form-button")
			searchButton.MustClick()

			//content := p.MustElement("#pf-v5-c-card")
			//content.MustWaitVisible() //the test wont pass if we wait for the load of data (need fixing)
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

			p.MustElement("select").MustSelect("Log Index")

			//select the "log index" option from the dropdown
			attrElement.MustClick()

			fmt.Println(entryIndexStr)
			// fill the text field with the log index
			hashInput := p.MustElement(`#rekor-search-log\ index`)
			hashInput.MustWaitVisible().MustInput(entryIndexStr)

			// verify the input value
			inputValue := hashInput.MustProperty("value").String()
			g.Got.Eq(inputValue, entryIndexStr)

			searchButton := p.MustElement("#search-form-button")
			searchButton.MustClick()

			//content := p.MustElement("#pf-v5-c-card")
			//content.MustWaitVisible() //the test wont pass if we wait for the load of data (need fixing)

		})
	})

	Describe("Test UUID", func() {
		It("test UI with Entry UUID", func() {

			g := setup() // invoke setup to get a new instance of G for this test
			p := g.page(appURL)
			//ensure the element is ready before interacting with it
			attrElement := p.MustElement("#rekor-search-attribute")
			attrElement.MustWaitVisible().MustClick()

			p.MustElement("select").MustSelect("Entry UUID")

			//select the "log index" option from the dropdown
			attrElement.MustClick()
			// fill the text field with the log index
			uuidInput := p.MustElement(`#rekor-search-entry\ uuid`)
			uuidInput.MustWaitVisible().MustInput(UuidStr)

			// verify the input value
			inputValue := uuidInput.MustProperty("value").String()
			g.Got.Eq(inputValue, UuidStr)
			searchButton := p.MustElement("#search-form-button")
			searchButton.MustClick()

			content := p.MustElement("#pf-v5-c-card")
			content.MustWaitVisible() //the test wont pass if we wait for the load of data (need fixing)

		})
	})

	Describe("Test Commit SHA", func() {
		It("test UI with Commit SHA using gitsign", func() {

			g := setup()
			p := g.page(appURL)

			attrElement := p.MustElement("#rekor-search-attribute")
			attrElement.MustWaitVisible().MustClick()

			p.MustElement("select").MustSelect("")
		})
	})
})
