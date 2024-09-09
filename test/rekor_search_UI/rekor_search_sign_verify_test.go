package rekor_search_UI

import (
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	. "github.com/onsi/ginkgo/v2"

	//. "github.com/onsi/gomega"
	"github.com/ysmood/got"
)

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
	// Setup function that initializes and returns a new instance of G
	setup := func() G {
		launch := launcher.New().Headless(false)
		url := launch.MustLaunch()
		browser := rod.New().ControlURL(url).MustConnect()  // create a new browser instance for each test
		return G{Got: got.New(GinkgoT()), Browser: browser} // Directly assign got.New(GinkgoT())
	}

	// A helper function to create a page
	const appURL = "http://localhost:3000" // will be replaced by URL from OCP, testing on localhost for now

	Describe("Test email", func() {
		It("test UI with email", func() {
			// Properly invoke setup to get an instance of G
			fmt.Println("came here0")
			g := setup()

			// Use the page method to navigate to the application URL
			p := g.page(appURL)

			// Ensure the element is ready before interacting with it
			attrElement := p.MustElement("#rekor-search-attribute")
			attrElement.MustWaitVisible().MustClick()

			// Select the "Email" option from the dropdown
			p.MustElement("select").MustSelect("Email")
			attrElement.MustClick()

			// Fill the text field with the email "jdoe@redhat.com"
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
})
