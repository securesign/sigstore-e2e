package rekor_search_UI

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"

	//. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/clients"
	"github.com/securesign/sigstore-e2e/test/testsupport"
	"github.com/sirupsen/logrus"
	"github.com/ysmood/got"
)

type G struct {
	got.G

	browser *rod.Browser
}

type helping struct {
	hashWithAlg string
}

var (
	rekorCli *clients.RekorCli
)
var RekorHash string
var hashWithAlg string
var entryIndex int

// helper function for stack values
var helper = func() {
	parseOutput := func(output string) testsupport.RekorCLIVerifyOutput {
		var rekorVerifyOutput testsupport.RekorCLIVerifyOutput
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if line == "" {
				continue // Skip empty lines
			}
			fields := strings.SplitN(line, ": ", 2) // Split by ": "
			if len(fields) == 2 {
				key := strings.TrimSpace(fields[0])
				value := strings.TrimSpace(fields[1])
				switch key {
				case "Entry Hash":
					rekorVerifyOutput.RekorHash = value
				case "Entry Index":
					entryIndex, err := strconv.Atoi(value)
					if err != nil {
						// Handle error
						fmt.Println("Error converting Entry Index to int:", err)
						return rekorVerifyOutput
					}
					rekorVerifyOutput.EntryIndex = entryIndex
				}
			}
		}
		return rekorVerifyOutput
	}
	rekorServerURL := api.GetValueFor(api.RekorURL)
	rekorKey := "ec_public.pem"
	output, err := rekorCli.CommandOutput(testsupport.TestContext, "verify", "--rekor_server", rekorServerURL, "--pki-format=x509", "--public-key", rekorKey)
	Expect(err).ToNot(HaveOccurred())
	logrus.Info(string(output))
	outputString := string(output)
	verifyOutput := parseOutput(outputString)
	RekorHash = "adsasdd"
	entryIndex = verifyOutput.EntryIndex
	var rekorGetOutput testsupport.RekorCLIGetOutput
	hashWithAlg = rekorGetOutput.RekordObj.Data.Hash.Algorithm + ":" + rekorGetOutput.RekordObj.Data.Hash.Value

}

var setup = func() func(t *testing.T) G {
	return func(t *testing.T) G {
		launch := launcher.New().Headless(false)
		url := launch.MustLaunch()
		browser := rod.New().ControlURL(url).MustConnect() // create a new browser instance for each test
		//t.Parallel()                                       // run each test concurrently if we need to speed up test
		return G{got.New(t), browser}
	}
}

// a helper function to create page
func (g G) page(url string) *rod.Page {
	page := g.browser.MustPage(url)
	g.Cleanup(func() {
		if err := page.Close(); err != nil {
			g.Logf("Failed to close page: %v", err)
		}
	})
	return page
}

const appURL = "http://localhost:3000" //will be replaced by URL from ocp, testing on localhost for now

// test for email
func TestEmail(t *testing.T) {
	fmt.Println("came here0")
	g := setup()(t) // invoke setup to get a new instance of G for this test
	p := g.page(appURL)
	//defer p.Close()
	//ensure the element is ready before interacting with it
	attrElement := p.MustElement("#rekor-search-attribute")
	attrElement.MustWaitVisible().MustClick()

	p.MustElement("select").MustSelect("Email")
	//select the "email" option from the dropdown
	attrElement.MustClick()

	// fill the text field with the email "jdoe@redhat.com"
	emailInput := p.MustElement("#rekor-search-email")
	emailInput.MustWaitVisible().MustInput("jdoe@redhat.com") //will be replaced with actual email used for signing

	// verify the input value
	inputValue := emailInput.MustProperty("value").String()
	g.Eq(inputValue, "jdoe@redhat.com")

	searchButton := p.MustElement("#search-form-button")
	searchButton.MustClick()
	content := p.MustElement("#pf-v5-c-card")
	content.MustWaitVisible() //the test wont pass if we wait for the load of data (need fixing)

}

// test for Hash
func TestHash(t *testing.T) {
	// h := helping{hashWithAlg: "1234567890abcdef"}

	//need to take hash from the stack so I can test it in UI

	g := setup()(t) // invoke setup to get a new instance of G for this test
	p := g.page(appURL)
	//ensure the element is ready before interacting with it
	attrElement := p.MustElement("#rekor-search-attribute")
	attrElement.MustWaitVisible().MustClick()

	//_ = p.MustElement("select").Select([]string{`[value="hash"]`}, false, rod.SelectorTypeCSSSector)
	p.MustElement("select").MustSelect("Hash")

	//select the "hash" option from the dropdown
	attrElement.MustClick()

	// fill the text field with the hash
	hashInput := p.MustElement("#rekor-search-hash")
	hashInput.MustWaitVisible().MustInput(hashWithAlg) //will be replaced with actual hash used for signing
	fmt.Println("came here5")

	// verify the input value
	inputValue := hashInput.MustProperty("value").String()
	g.Eq(inputValue, hashWithAlg) //here we take the hash from stack
	fmt.Println("came here6")

	searchButton := p.MustElement("#search-form-button")
	searchButton.MustClick()

	content := p.MustElement("#pf-v5-c-card")
	content.MustWaitVisible() //the test wont pass if we wait for the load of data (need fixing)
}

// test for commit SHA
func TestSHA(t *testing.T) {
	//need to take hash from the stack so I can test it in UI

	g := setup()(t) // invoke setup to get a new instance of G for this test
	p := g.page(appURL)
	//ensure the element is ready before interacting with it
	attrElement := p.MustElement("#rekor-search-attribute")
	attrElement.MustWaitVisible().MustClick()

	p.MustElement("select").MustSelect("Commit SHA")

	//select the "hash" option from the dropdown
	attrElement.MustClick()

	// fill the text field with the hash
	hashInput := p.MustElement("#rekor-search-commit sha")
	hashInput.MustWaitVisible().MustInput(RekorHash) //will be replaced with actual hash used for signing
	fmt.Println("came here5")

	// verify the input value
	//!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
	//change the values
	//!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
	inputValue := hashInput.MustProperty("value").String()
	g.Eq(inputValue, RekorHash) //here we take the hash from stack
	fmt.Println("came here6")

	searchButton := p.MustElement("#search-form-button")
	searchButton.MustClick()
}
