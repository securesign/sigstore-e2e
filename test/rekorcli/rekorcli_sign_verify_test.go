package rekorcli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
			Expect(rekorCli.Command(testsupport.TestContext, "upload", "--rekor_server", rekorServerURL, "--signature", signatureFilePath, "--pki-format=x509", "--public-key", rekorKey, "--artifact", tarFilePath).Run()).To(Succeed())
		})
	})

	Describe("Verify upload", func() {
		It("should verify uploaded artifact", func() {
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
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "verify", "--rekor_server", rekorServerURL, "--signature", signatureFilePath, "--pki-format=x509", "--public-key", rekorKey, "--artifact", tarFilePath)
			Expect(err).ToNot(HaveOccurred())
			outputString := string(output)
			verifyOutput := parseOutput(outputString)
			rekorHash = verifyOutput.RekorHash
			entryIndex = verifyOutput.EntryIndex
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
			Expect(rekorCli.Command(testsupport.TestContext, "loginfo", "--rekor_server", rekorServerURL).Run()).To(Succeed())
		})
	})

	Describe("Search entries", func() {
		It("should search entries with artifact ", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			Expect(rekorCli.Command(testsupport.TestContext, "search", "--rekor_server", rekorServerURL, "--artifact", tarFilePath).Run()).To(Succeed())
		})
	})

	Describe("Search entries", func() {
		It("should search entries with public key", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			rekorKey := "ec_public.pem"
			Expect(rekorCli.Command(testsupport.TestContext, "search", "--rekor_server", rekorServerURL, "--public-key", rekorKey, "--pki-format=x509").Run()).To(Succeed())
		})

	})

	Describe("Search entries", func() {
		It("should search entries with hash", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			Expect(rekorCli.Command(testsupport.TestContext, "search", "--rekor_server", rekorServerURL, "--sha", hashWithAlg).Run()).To(Succeed())
		})
	})
})

var _ = AfterSuite(func() {
	// Cleanup shared resources after all tests have run.
	Expect(os.RemoveAll(tempDir)).To(Succeed())
})
