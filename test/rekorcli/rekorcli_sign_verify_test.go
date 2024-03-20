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

type rekorCliVerifyOutput struct {
	RekorHash  string
	EntryIndex int
}

type RekorCLIOutput struct {
	RekordObj struct {
		Data struct {
			Hash struct {
				Algorithm string `json:"algorithm"`
				Value     string `json:"value"`
			} `json:"hash"`
		} `json:"data"`
		Signature struct {
			Content   string `json:"content"`
			PublicKey struct {
				Content string `json:"content"`
			} `json:"publicKey"`
		} `json:"signature"`
	} `json:"RekordObj"`
}

var _ = Describe("Verify entries, query the transparency log for inclusion proof", Ordered, func() {

	var (
		err       error
		rekorCli  *clients.RekorCli
		rekorHash string
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

		// create directory and tar it
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
			parseOutput := func(output string) rekorCliVerifyOutput {
				var result rekorCliVerifyOutput
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
							result.RekorHash = value
						case "Entry Index":
							entryIndex, err := strconv.Atoi(value)
							if err != nil {
								// Handle error
								fmt.Println("Error converting Entry Index to int:", err)
								return result
							}
							result.EntryIndex = entryIndex
						}
					}
				}
				return result
			}

			rekorServerURL := api.GetValueFor(api.RekorURL)
			rekorKey := "ec_public.pem"
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "verify", "--rekor_server", rekorServerURL, "--signature", signatureFilePath, "--pki-format=x509", "--public-key", rekorKey, "--artifact", tarFilePath)
			Expect(err).ToNot(HaveOccurred())
			logrus.Info(string(output))
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

			logrus.Info(string(output))
			startIndex := strings.Index(string(output), "{")
			if startIndex == -1 {
				// Handle error: JSON start not found
				return
			}
			jsonStr := string(output[startIndex:])
			var result RekorCLIOutput
			err = json.Unmarshal([]byte(jsonStr), &result)
			Expect(err).ToNot(HaveOccurred())

			// algorithm:hashValue
			hashWithAlg = result.RekordObj.Data.Hash.Algorithm + ":" + result.RekordObj.Data.Hash.Value
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
