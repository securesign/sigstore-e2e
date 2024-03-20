package cosign

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"path/filepath"
	
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
var tempDir string
var publicKeyPath string
var signaturePath string

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

		Expect(testsupport.InstallPrerequisites(cosign, rekorCli)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

		tempDir, err = os.MkdirTemp("", "rekorTest")
		Expect(err).ToNot(HaveOccurred())

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
			Expect(cosign.Command(testsupport.TestContext, "sign", "-y", "--identity-token="+token, targetImageName).Run()).To(Succeed())
		})
	})

	Describe("cosign verify", func() {
		It("should verify the signature and extract logIndex", func() {
			output, err := cosign.CommandOutput(testsupport.TestContext, "verify", "--certificate-identity-regexp", ".*@redhat", "--certificate-oidc-issuer-regexp", ".*keycloak.*", targetImageName)
			Expect(err).ToNot(HaveOccurred())

			logrus.Info(string(output))

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
			rekorServerURL := api.GetValueFor(api.RekorURL)
			logIndexStr := strconv.Itoa(logIndex)

			output, err := rekorCli.CommandOutput(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--log-index", logIndexStr)
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

			signatureContent := result.HashedRekordObj.Signature.Content
			publicKeyContent := result.HashedRekordObj.Signature.PublicKey.Content
			hashValue = result.HashedRekordObj.Data.Hash.Value

			decodedSignatureContent, err := base64.StdEncoding.DecodeString(signatureContent)
			Expect(err).ToNot(HaveOccurred())

			// Decode publicKeyContent from base64
			decodedPublicKeyContent, err := base64.StdEncoding.DecodeString(publicKeyContent)
			Expect(err).ToNot(HaveOccurred())

			publicKeyPath = filepath.Join(tempDir, "publickey.pem")
			signaturePath = filepath.Join(tempDir, "signature.bin")

			Expect(os.WriteFile(publicKeyPath, decodedPublicKeyContent, 0600)).To(Succeed())
			Expect(os.WriteFile(signaturePath, decodedSignatureContent, 0600)).To(Succeed())
		})
	})

	Describe("rekor-cli verify artifact", func() {
		It("should verify the artifact using rekor-cli", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)

			Expect(rekorCli.Command(testsupport.TestContext, "verify", "--rekor_server", rekorServerURL, "--signature", signaturePath, "--public-key", publicKeyPath, "--pki-format", "x509", "--type", "hashedrekord:0.0.1", "--artifact-hash", hashValue).Run()).To(Succeed())
		})
	})
})

var _ = AfterSuite(func() {
    // Cleanup shared resources after all tests have run.
    Expect(os.RemoveAll(tempDir)).To(Succeed())
})