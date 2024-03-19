package rekorcli

import (
	"fmt"
	"os"
	"os/exec"
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

//var prefix string

type rekorCliVerifyOutput struct {
	RekorHash  string
	EntryIndex int
}

type rekorHashOutput struct {
	HashedRekordObj struct {
		Data struct {
			Hash struct {
				Prefix string `json:"algorithm"`
				Value  string `json:"value"`
			} `json:"hash"`
		} `json:"data"`
		Signature struct {
			Content   string `json:"content"`
			PublicKey struct {
				Content string `json:"content"`
			} `json:"publicKey"`
		} `json:"signature"`
	} `json:"HashedRekordObj"`
}

var _ = Describe("Verify entries, query the transparency log for inclusion proof", Ordered, func() {

	var (
		err       error
		rekorCli  *clients.RekorCli
		rekorHash string
		//hashValue string
		//prefix string
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

		//create directory and tar it
		var dir = "myrelease"
		err := os.Mkdir(dir, 0755) // 0755 = the folder will be readable and executed by others, but writable by the user only
		if err != nil {
			panic(err) // handle error
		}
		//now taring it for release
		var tar = "myrelease.tar.gz"
		tarCmd := exec.Command("tar", "-czvf", tar, dir)
		err = tarCmd.Run()
		if err != nil {
			panic(err) // handle error
		}

		//sign artifact with public key
		sign := "mysignature.asc"
		opensslKey := exec.Command("openssl", "dgst", "-sha256", "-sign", "ec_private.pem", "-out", sign, tar)
		err = opensslKey.Run()
		if err != nil {
			panic(err)
		}

	})

	Describe("Upload artifact", func() {
		It("should upload artifact", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			rekorKey := "ec_public.pem"
			Expect(rekorCli.Command(testsupport.TestContext, "upload", "--rekor_server", rekorServerURL, "--signature", "mysignature.asc", "--pki-format=x509", "--public-key", rekorKey, "--artifact", "myrelease.tar.gz").Run()).To(Succeed())
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
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "verify", "--rekor_server", rekorServerURL, "--signature", "mysignature.asc", "--pki-format=x509", "--public-key", rekorKey, "--artifact", "myrelease.tar.gz")
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
			Expect(rekorCli.Command(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--uuid", rekorHash).Run()).To(Succeed()) //UUID = Entry Hash here
			Expect(err).ToNot(HaveOccurred())

		})

	})

	Describe("Get with logindex", func() {
		It("should get data from rekor server", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			entryIndexStr := strconv.Itoa(entryIndex)
			//extrract of hash value for searching with --sha
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--log-index", entryIndexStr)
			Expect(err).ToNot(HaveOccurred())

			logrus.Info(string(output))
			startIndex := strings.Index(string(output), "{ ")
			if startIndex == -1 {
				// Handle error: JSON start not found
				return
			}

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
			Expect(rekorCli.Command(testsupport.TestContext, "search", "--rekor_server", rekorServerURL, "--artifact", "myrelease.tar.gz").Run()).To(Succeed())

		})
	})

	Describe("Search entries", func() {
		It("should search entries with public key", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			rekorKey := "ec_public.pem"
			Expect(rekorCli.Command(testsupport.TestContext, "search", "--rekor_server", rekorServerURL, "--public-key", rekorKey, "--pki-format=x509").Run()).To(Succeed())
		})
		AfterEach(func() {

			err := os.Remove("myrelease")
			Expect(err).ToNot(HaveOccurred())

			err = os.Remove("myrelease.tar.gz")
			Expect(err).ToNot(HaveOccurred())

			err = os.Remove("mysignature.asc")
			Expect(err).ToNot(HaveOccurred())

		})

	})

	// Describe("Search entries", func() {
	// 	It("should search entries with hash", func() {
	// 		rekorServerURL := api.GetValueFor(api.RekorURL)
	// 		Expect(rekorCli.Command(testsupport.TestContext, "search", "--rekor_server", rekorServerURL, "--sha", hashValue).Run()).To(Succeed())
	// 	})

	// })
})
