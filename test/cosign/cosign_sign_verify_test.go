package cosign

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/image"

	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/clients"
	"github.com/securesign/sigstore-e2e/test/testsupport"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

const testImage string = "mirror.gcr.io/alpine:latest"

var logIndex int
var hashValue string
var rekorEntryType string
var dsseEnvelopePath string
var tempDir string
var publicKeyPath string
var signaturePath string
var predicatePath string
var targetImageName string

var _ = Describe("Cosign test", Ordered, func() {

	var (
		err       error
		dockerCli *client.Client
		cosign    *clients.Cosign
		rekorCli  *clients.RekorCli
		ec        *clients.EnterpriseContract
	)

	BeforeAll(func() {
		logrus.Infof("Starting cosign test")
		err = testsupport.CheckMandatoryAPIConfigValues(api.OidcRealm)
		if err != nil {
			Fail(err.Error())
		}

		cosign = clients.NewCosign()

		rekorCli = clients.NewRekorCli()

		ec = clients.NewEnterpriseContract()

		Expect(testsupport.InstallPrerequisites(cosign, rekorCli, ec)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

		tempDir, err = os.MkdirTemp("", "tmp")
		Expect(err).ToNot(HaveOccurred())

		manualImageSetup := api.GetValueFor(api.ManualImageSetup) == "true"
		if !manualImageSetup {
			targetImageName = "ttl.sh/" + uuid.New().String() + ":5m"
			dockerCli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			Expect(err).ToNot(HaveOccurred())

			var pull io.ReadCloser
			pull, err = dockerCli.ImagePull(testsupport.TestContext, testImage, image.PullOptions{})
			Expect(err).ToNot(HaveOccurred())
			_, err = io.Copy(os.Stdout, pull)
			Expect(err).ToNot(HaveOccurred())
			defer pull.Close()

			Expect(dockerCli.ImageTag(testsupport.TestContext, testImage, targetImageName)).To(Succeed())
			var push io.ReadCloser
			// use empty auth to avoid  "invalid X-Registry-Auth header: EOF" (https://github.com/moby/moby/issues/10983
			push, err = dockerCli.ImagePush(testsupport.TestContext, targetImageName, image.PushOptions{RegistryAuth: base64.StdEncoding.EncodeToString([]byte("{}"))})
			Expect(err).ToNot(HaveOccurred())
			_, err = io.Copy(os.Stdout, push)
			Expect(err).ToNot(HaveOccurred())
			defer push.Close()
		} else {
			targetImageName = api.GetValueFor(api.TargetImageName)
			Expect(targetImageName).NotTo(BeEmpty(), "TARGET_IMAGE_NAME environment variable must be set when MANUAL_IMAGE_SETUP is true")
		}
	})

	Describe("Cosign initialize", func() {
		It("should initialize the TUF root", func() {
			Expect(cosign.Command(testsupport.TestContext, "initialize").Run()).To(Succeed())
		})
	})

	Describe("cosign sign", func() {
		It("should sign the container", func() {
			token, err := testsupport.GetOIDCToken(testsupport.TestContext)
			Expect(err).ToNot(HaveOccurred())

			Expect(cosign.Command(testsupport.TestContext, "sign", "--identity-token="+token, targetImageName).Run()).To(Succeed())

			// Extract logIndex by downloading signature bundles from the registry.
			// Multiple bundles may exist if the image was signed more than once;
			// pick the one with the highest logIndex (most recent).
			bundleOutput, err := cosign.CommandOutput(testsupport.TestContext, "download", "signature", targetImageName)
			Expect(err).ToNot(HaveOccurred())

			startIdx := strings.Index(string(bundleOutput), "{")
			Expect(startIdx).NotTo(Equal(-1), "JSON start - '{' not found in bundle output")

			type sigBundle struct {
				VerificationMaterial struct {
					TlogEntries []struct {
						LogIndex json.RawMessage `json:"logIndex"`
					} `json:"tlogEntries"`
				} `json:"verificationMaterial"`
				DSSEEnvelope json.RawMessage `json:"dsseEnvelope"`
			}

			decoder := json.NewDecoder(strings.NewReader(string(bundleOutput[startIdx:])))
			logIndex = -1
			var bestBundle sigBundle
			for decoder.More() {
				var b sigBundle
				if err := decoder.Decode(&b); err != nil {
					break
				}
				if len(b.VerificationMaterial.TlogEntries) == 0 {
					continue
				}
				idx, err := strconv.Atoi(strings.Trim(string(b.VerificationMaterial.TlogEntries[0].LogIndex), "\""))
				if err != nil {
					continue
				}
				if idx > logIndex {
					logIndex = idx
					bestBundle = b
				}
			}
			Expect(logIndex).To(BeNumerically(">=", 0), "no valid signature bundle found")

			if len(bestBundle.DSSEEnvelope) > 0 {
				dsseEnvelopePath = filepath.Join(tempDir, "dsse-envelope.json")
				Expect(os.WriteFile(dsseEnvelopePath, bestBundle.DSSEEnvelope, 0600)).To(Succeed())
			}
		})
	})

	Describe("cosign verify", func() {
		It("should verify the signature", func() {
			_, err := cosign.CommandOutput(testsupport.TestContext, "verify", "--certificate-identity-regexp", ".*"+regexp.QuoteMeta(api.GetValueFor(api.OidcUserDomain)), "--certificate-oidc-issuer-regexp", regexp.QuoteMeta(api.GetValueFor(api.OidcIssuerURL)), targetImageName)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("rekor-cli get (via --log-index)", func() {
		It("should retrieve the entry from Rekor and create public-key and signature files", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)
			logIndexStr := strconv.Itoa(logIndex)

			output, err := rekorCli.CommandOutput(testsupport.TestContext, "get", "--rekor_server", rekorServerURL, "--log-index", logIndexStr)
			Expect(err).ToNot(HaveOccurred())

			// Look for JSON start
			startIndex := strings.Index(string(output), "{")
			Expect(startIndex).NotTo(Equal(-1), "JSON start - '{' not found")

			jsonStr := string(output[startIndex:])

			var rekorGetOutput testsupport.RekorCLIGetOutput
			err = json.Unmarshal([]byte(jsonStr), &rekorGetOutput)
			Expect(err).ToNot(HaveOccurred())

			// Extract values from rekor-cli get output - handle both HashedRekordObj and DSSEObj
			var signatureContent, publicKeyContent string
			if len(rekorGetOutput.DSSEObj.Signatures) > 0 {
				rekorEntryType = "dsse:0.0.1"
				signatureContent = rekorGetOutput.DSSEObj.Signatures[0].Signature
				publicKeyContent = rekorGetOutput.DSSEObj.Signatures[0].Verifier
				hashValue = rekorGetOutput.DSSEObj.PayloadHash.Value
			} else if rekorGetOutput.HashedRekordObj.Signature.Content != "" {
				rekorEntryType = "hashedrekord:0.0.1"
				signatureContent = rekorGetOutput.HashedRekordObj.Signature.Content
				publicKeyContent = rekorGetOutput.HashedRekordObj.Signature.PublicKey.Content
				hashValue = rekorGetOutput.HashedRekordObj.Data.Hash.Value
			} else if rekorGetOutput.RekordObj.Signature.Content != "" {
				rekorEntryType = "rekord:0.0.1"
				signatureContent = rekorGetOutput.RekordObj.Signature.Content
				publicKeyContent = rekorGetOutput.RekordObj.Signature.PublicKey.Content
				hashValue = rekorGetOutput.RekordObj.Data.Hash.Value
			} else {
				Fail("Unrecognized Rekor entry type in rekor-cli get output")
			}

			// Decode signatureContent and publicKeyContent from base64
			decodedSignatureContent, err := base64.StdEncoding.DecodeString(signatureContent)
			Expect(err).ToNot(HaveOccurred())

			decodedPublicKeyContent, err := base64.StdEncoding.DecodeString(publicKeyContent)
			Expect(err).ToNot(HaveOccurred())

			// Create files in the tempDir
			publicKeyPath = filepath.Join(tempDir, "publickey.pem")
			signaturePath = filepath.Join(tempDir, "signature.bin")

			Expect(os.WriteFile(publicKeyPath, decodedPublicKeyContent, 0600)).To(Succeed())
			Expect(os.WriteFile(signaturePath, decodedSignatureContent, 0600)).To(Succeed())
		})
	})

	Describe("rekor-cli verify", func() {
		It("should verify the artifact using rekor-cli", func() {
			rekorServerURL := api.GetValueFor(api.RekorURL)

			if rekorEntryType == "dsse:0.0.1" {
				// DSSE entries require the full envelope as --artifact
				Expect(rekorCli.Command(testsupport.TestContext, "verify", "--rekor_server", rekorServerURL, "--artifact", dsseEnvelopePath, "--public-key", publicKeyPath, "--pki-format", "x509", "--type", rekorEntryType).Run()).To(Succeed())
			} else {
				Expect(rekorCli.Command(testsupport.TestContext, "verify", "--rekor_server", rekorServerURL, "--signature", signaturePath, "--public-key", publicKeyPath, "--pki-format", "x509", "--type", rekorEntryType, "--artifact-hash", hashValue).Run()).To(Succeed())
			}
		})
	})

	Describe("cosign attest", func() {
		It("should create a predicate.json file", func() {
			predicateJSONContent := `{
				"builder": {
					"id": "https://localhost/dummy-id"
				},
				"buildType": "https://example.com/tekton-pipeline",
				"invocation": {},
				"buildConfig": {},
				"metadata": {
					"completeness": {
						"parameters": false,
						"environment": false,
						"materials": false
					},
					"reproducible": false
				},
				"materials": []
			}`

			predicatePath = filepath.Join(tempDir, "predicate.json")

			Expect(os.WriteFile(predicatePath, []byte(predicateJSONContent), 0600)).To(Succeed())
		})

		It("should sign and attach the predicate as an attestation to the image", func() {
			token, err := testsupport.GetOIDCToken(testsupport.TestContext)
			Expect(err).ToNot(HaveOccurred())

			Expect(cosign.Command(testsupport.TestContext, "attest", "--identity-token="+token, "--predicate", predicatePath, "--type", "slsaprovenance", targetImageName).Run()).To(Succeed())
		})
	})

	Describe("cosign tree", func() {
		It("should verify that the container image has at least one attestation and signature", func() {
			output, err := cosign.CommandOutput(testsupport.TestContext, "tree", targetImageName)
			Expect(err).ToNot(HaveOccurred())

			// Matching (generic) hash entries
			hashPattern := regexp.MustCompile(`└── 🍒 \w+:[0-9a-f]{64}`)

			lines := strings.Split(string(output), "\n")

			usesOCIReferrers := false
			for _, line := range lines {
				if strings.Contains(line, "OCI referrer") || strings.Contains(line, "via OCI") {
					usesOCIReferrers = true
					break
				}
			}

			if usesOCIReferrers {
				artifactCount := 0
				for _, line := range lines {
					if hashPattern.MatchString(line) {
						artifactCount++
					}
				}
				Expect(artifactCount).To(BeNumerically(">=", 2),
					"Expected at least 2 artifacts (signature + attestation) in cosign tree output")
			} else {
				inSignatureSection := false
				inAttestationSection := false
				hasSignature := false
				hasAttestation := false

				for _, line := range lines {
					if strings.Contains(line, "Signatures for an image tag:") {
						inSignatureSection = true
						inAttestationSection = false
						continue
					} else if strings.Contains(line, "Attestations for an image tag:") {
						inSignatureSection = false
						inAttestationSection = true
						continue
					}

					if inSignatureSection && hashPattern.MatchString(line) {
						hasSignature = true
					} else if inAttestationSection && hashPattern.MatchString(line) {
						hasAttestation = true
					}
				}

				Expect(hasAttestation).To(BeTrue(), "Expected the image to have at least one attestation")
				Expect(hasSignature).To(BeTrue(), "Expected the image to have at least one signature")
			}
		})
	})

	// TODO: SECURESIGN-3841
	Describe("ec validate [optional]", Pending, func() {
		It("should initialize ec TUF root", func() {
			tufURL := api.GetValueFor(api.TufURL)
			Expect(ec.Command(testsupport.TestContext, "sigstore", "initialize",
				"--mirror", tufURL,
				"--root", tufURL+"/root.json").Run()).To(Succeed())
		})

		It("should verify signature and attestation of the image", func() {
			output, err := ec.CommandOutput(testsupport.TestContext, "validate", "image", "--image", targetImageName, "--certificate-identity-regexp", ".*"+regexp.QuoteMeta(api.GetValueFor(api.OidcUserDomain)), "--certificate-oidc-issuer-regexp", ".*"+regexp.QuoteMeta(api.GetValueFor(api.OidcIssuerURL)), "--output", "yaml", "--show-successes")
			Expect(err).ToNot(HaveOccurred())

			successPatterns := []*regexp.Regexp{
				regexp.MustCompile(`success: true\s+successes:`),
				regexp.MustCompile(`metadata:\s+code: builtin.attestation.signature_check\s+msg: Pass`),
				regexp.MustCompile(`metadata:\s+code: builtin.attestation.syntax_check\s+msg: Pass`),
				regexp.MustCompile(`metadata:\s+code: builtin.image.signature_check\s+msg: Pass`),
				regexp.MustCompile(`ec-version:`),
				regexp.MustCompile(`effective-time:`),
				regexp.MustCompile(`key: ""\s+policy: {}\s+success: true`),
			}

			for _, pattern := range successPatterns {
				Expect(pattern.Match(output)).To(BeTrue(), "Expected to find success message matching: %s", pattern.String())
			}
		})
	})
})

var _ = AfterSuite(func() {
	// Cleanup shared resources after all tests have run.
	Expect(os.RemoveAll(tempDir)).To(Succeed())
})
