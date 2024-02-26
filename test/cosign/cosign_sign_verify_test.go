package cosign

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/clients"
	"github.com/securesign/sigstore-e2e/test/testsupport"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

const testImage string = "alpine:latest"

var logIndex int
var hashValue string

type CosignVerificationOutput []struct {
	Optional struct {
		Bundle struct {
			Payload struct {
				LogIndex int `json:"logIndex"`
			} `json:"Payload"`
		} `json:"Bundle"`
	} `json:"optional"`
}

type RekorCLIOutput struct {
	HashedRekordObj struct {
		Data struct {
			Hash struct {
				Value string `json:"value"`
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

var _ = Describe("Cosign test", Ordered, func() {

	var (
		err       error
		dockerCli *client.Client
		cosign    *clients.Cosign
		rekorCli  *clients.RekorCli
	)
	targetImageName := "ttl.sh/" + uuid.New().String() + ":5m"

	BeforeAll(func() {
		err = testsupport.CheckMandatoryAPIConfigValues(api.OidcRealm)
		if err != nil {
			Skip("Skip this test - " + err.Error())
		}

		cosign = clients.NewCosign()

		rekorCli = clients.NewRekorCli()

		Expect(testsupport.InstallPrerequisites(
			cosign,
			rekorCli,
		)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

		dockerCli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		Expect(err).ToNot(HaveOccurred())

		var pull io.ReadCloser
		pull, err = dockerCli.ImagePull(testsupport.TestContext, testImage, types.ImagePullOptions{})
		Expect(err).ToNot(HaveOccurred())
		_, err = io.Copy(os.Stdout, pull)
		Expect(err).ToNot(HaveOccurred())
		defer pull.Close()

		Expect(dockerCli.ImageTag(testsupport.TestContext, testImage, targetImageName)).To(Succeed())
		var push io.ReadCloser
		push, err = dockerCli.ImagePush(testsupport.TestContext, targetImageName, types.ImagePushOptions{})
		Expect(err).ToNot(HaveOccurred())
		_, err = io.Copy(os.Stdout, push)
		Expect(err).ToNot(HaveOccurred())
		defer push.Close()
		// wait for a while to be sure that the image landed in the registry
		time.Sleep(10 * time.Second)
	})

	Describe("Cosign initialize", func() {
		It("should initialize the cosign root", func() {
			Expect(cosign.Command(testsupport.TestContext, "initialize").Run()).To(Succeed())
		})
	})

	Describe("cosign sign", func() {
		It("should sign the container", func() {
			token, err := testsupport.GetOIDCToken(testsupport.TestContext, api.GetValueFor(api.OidcIssuerURL), "jdoe", "secure", api.GetValueFor(api.OidcRealm))
			Expect(err).ToNot(HaveOccurred())
			Expect(cosign.Command(testsupport.TestContext, "sign",
				"-y", "--identity-token="+token, targetImageName).Run()).To(Succeed())
		})
	})

	Describe("cosign verify", func() {
		It("should verify the signature and extract logIndex", func() {
			output, err := cosign.CommandOutput(testsupport.TestContext, "verify", "--certificate-identity-regexp", ".*@redhat", "--certificate-oidc-issuer-regexp", ".*keycloak.*", targetImageName)
			Expect(err).ToNot(HaveOccurred())

			startIndex := strings.Index(string(output), "[")
			if startIndex == -1 {
				// Handle error: JSON start not found
				return
			}
			jsonStr := string(output[startIndex:])

			var verificationOutput CosignVerificationOutput
			err = json.Unmarshal([]byte(jsonStr), &verificationOutput)
			Expect(err).ToNot(HaveOccurred())

			logIndex = verificationOutput[0].Optional.Bundle.Payload.LogIndex
		})
	})

	Describe("rekor-cli get with logIndex", func() {
		It("should retrieve the entry from Rekor", func() {
			// Assuming `logIndex` is obtained from previous tests or steps
			rekorServerURL := api.GetValueFor(api.RekorURL)
			logIndexStr := strconv.Itoa(logIndex)

			output, err := rekorCli.CommandOutput(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--log-index", logIndexStr)
			Expect(err).ToNot(HaveOccurred())

			startIndex := strings.Index(string(output), "{")
			if startIndex == -1 {
				// Handle error: JSON start not found
				return
			}
			jsonStr := string(output[startIndex:])

			var result RekorCLIOutput
			err = json.Unmarshal([]byte(jsonStr), &result)
			Expect(err).ToNot(HaveOccurred())

			signatureContent := result.HashedRekordObj.Signature.Content
			publicKeyContent := result.HashedRekordObj.Signature.PublicKey.Content
			hashValue = result.HashedRekordObj.Data.Hash.Value

			decodedSignatureContent, err := base64.StdEncoding.DecodeString(signatureContent)
			Expect(err).ToNot(HaveOccurred())

			// Decode publicKeyContent from base64
			decodedPublicKeyContent, err := base64.StdEncoding.DecodeString(publicKeyContent)
			Expect(err).ToNot(HaveOccurred())

			err = os.WriteFile("publickey.pem", decodedPublicKeyContent, 0644) // 0644 provides read and write permissions to the owner, and read permission to others
			Expect(err).ToNot(HaveOccurred())

			// Write decoded signature content to signature.bin
			err = os.WriteFile("signature.bin", decodedSignatureContent, 0644)
			Expect(err).ToNot(HaveOccurred())

		})
	})

	Describe("Rekor CLI Verify Artifact", func() {
		It("should verify the artifact using rekor-cli", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL) // Ensure this is the correct way to retrieve your Rekor server URL
			// Ensure hashValue, signature.bin, and publickey.pem are available and correctly set up before this step.

			Expect(rekorCli.Command(testsupport.TestContext, "verify", "--rekor_server", rekorServerURL,
				"--signature", "signature.bin", "--public-key", "publickey.pem", "--pki-format", "x509", "--type",
				"hashedrekord:0.0.1", "--artifact-hash", hashValue).Run()).To(Succeed())
		})

		AfterEach(func() {
			// Attempt to remove signature.bin and publickey.pem after each test
			err := os.Remove("signature.bin")
			Expect(err).ToNot(HaveOccurred())

			err = os.Remove("publickey.pem")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
