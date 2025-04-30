package rekorcli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/clients"
	"github.com/securesign/sigstore-e2e/test/testsupport"
	"github.com/sirupsen/logrus"
)

var entryIndex int
var hashWithAlg string
var tempDir string
var dirFilePath string
var tarFilePath string
var signatureFilePath string

var _ = Describe("Verify entries, query the transparency log for inclusion proof", Ordered, func() {

	var (
		err       error
		rekorCli  *clients.RekorCli
		rekorHash string
	)

	BeforeAll(func() {
		err = testsupport.CheckMandatoryAPIConfigValues(api.OidcRealm)
		if err != nil {
			Fail(err.Error())
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

		// tempDir for tarball and signature
		tempDir, err = os.MkdirTemp("", "rekorTest")
		Expect(err).ToNot(HaveOccurred())

		dirFilePath = filepath.Join(tempDir, "myrelease")
		tarFilePath = filepath.Join(tempDir, "myrelease.tar.gz")
		signatureFilePath = filepath.Join(tempDir, "mysignature.asc")

		// create directory and tar it
		err := os.Mkdir(dirFilePath, 0755) // 0755 = the folder will be readable and executed by others, but writable by the user only
		if err != nil {
			panic(err) // handle error
		}

		// now taring it for release
		tarCmd := exec.Command("tar", "-czvf", tarFilePath, dirFilePath)
		err = tarCmd.Run()
		if err != nil {
			panic(err) // handle error
		}

		// sign artifact with public key
		opensslKey := exec.Command("openssl", "dgst", "-sha256", "-sign", "ec_private.pem", "-out", signatureFilePath, tarFilePath)
		err = opensslKey.Run()
		if err != nil {
			panic(err)
		}

	})

	Describe("Upload artifact", func() {
		It("should upload artifact", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			rekorKey := "ec_public.pem"
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "upload", "--rekor_server", rekorServerURL, "--signature", signatureFilePath, "--pki-format=x509", "--public-key", rekorKey, "--artifact", tarFilePath)
			Expect(err).ToNot(HaveOccurred())

			createdMessage := regexp.MustCompile(`Created entry at index (\d+)`)
			matches := createdMessage.FindStringSubmatch(string(output))
			Expect(matches).To(HaveLen(2))

			entryIndex, err = strconv.Atoi(matches[1])
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Verify upload", func() {
		It("should verify uploaded artifact", func() {
			parseOutput := func(output string) string {
				hashRe := regexp.MustCompile(`Entry Hash:\s+([a-f0-9]+)`)
				hashMatches := hashRe.FindStringSubmatch(output)
				Expect(hashMatches).To(HaveLen(2))

				return hashMatches[1]
			}

			rekorServerURL := api.GetValueFor(api.RekorURL)
			rekorKey := "ec_public.pem"
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "verify", "--rekor_server", rekorServerURL, "--signature", signatureFilePath, "--pki-format=x509", "--public-key", rekorKey, "--artifact", tarFilePath)
			Expect(err).ToNot(HaveOccurred())
			outputString := string(output)
			rekorHash = parseOutput(outputString)
		})
	})

	Describe("Verify entry consistency", func() {
		It("should use the same entry across tests", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--uuid", rekorHash)
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.Contains(string(output), strconv.Itoa(entryIndex))).To(BeTrue())
			Expect(strings.Contains(string(output), rekorHash)).To(BeTrue())

			output, err = rekorCli.CommandOutput(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--log-index", strconv.Itoa(entryIndex))
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.Contains(string(output), strconv.Itoa(entryIndex))).To(BeTrue())
			Expect(strings.Contains(string(output), rekorHash)).To(BeTrue())
		})
	})

	Describe("Get with UUID", func() {
		It("should get data from rekor server", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			Expect(rekorCli.Command(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--uuid", rekorHash).Run()).To(Succeed()) // UUID = Entry Hash here
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Get with logindex", func() {
		It("should get data from rekor server", func() {
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
		})
	})

	Describe("Get loginfo", func() {
		It("should get loginfo from rekor server", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "loginfo", "--rekor_server", rekorServerURL)
			Expect(err).ToNot(HaveOccurred())
			fmt.Println(output)
		})
	})

	Describe("Search entries", func() {
		It("should search entries with artifact ", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "search", "--rekor_server", rekorServerURL, "--artifact", tarFilePath)
			Expect(err).ToNot(HaveOccurred())
			fmt.Println(output)
		})
	})

	Describe("Search entries", func() {
		It("should search entries with public key", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			rekorKey := "ec_public.pem"
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "search", "--rekor_server", rekorServerURL, "--public-key", rekorKey, "--pki-format=x509")
			Expect(err).ToNot(HaveOccurred())
			fmt.Println(output)
		})

	})

	Describe("Search entries", func() {
		It("should search entries with hash", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "search", "--rekor_server", rekorServerURL, "--sha", hashWithAlg)
			Expect(err).ToNot(HaveOccurred())
			fmt.Println(output)
		})
	})

})

var _ = AfterSuite(func() {
	// Cleanup shared resources after all tests have run.
	Expect(os.RemoveAll(tempDir)).To(Succeed())
})
