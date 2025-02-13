package cosign

import (
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

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

const tsaTestImage string = "alpine:latest"

var tsaTargetImageName string
var tsaChainPath string

var _ = Describe("TSA test", Ordered, func() {

	var (
		err       error
		dockerCli *client.Client
		cosign    *clients.Cosign
	)

	BeforeAll(func() {
		logrus.Infof("Starting TSA cosign test")
		err = testsupport.CheckMandatoryAPIConfigValues(api.OidcRealm)
		if err != nil {
			Skip("Skip this test - " + err.Error())
		}

		cosign = clients.NewCosign()

		Expect(testsupport.InstallPrerequisites(cosign)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

		tempDir, err = os.MkdirTemp("", "tmp")
		Expect(err).ToNot(HaveOccurred())

		manualImageSetup := api.GetValueFor(api.ManualImageSetup) == "true"
		if !manualImageSetup {
			tsaTargetImageName = "ttl.sh/" + uuid.New().String() + ":5m"
			dockerCli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			Expect(err).ToNot(HaveOccurred())

			var pull io.ReadCloser
			pull, err = dockerCli.ImagePull(testsupport.TestContext, tsaTestImage, image.PullOptions{})
			Expect(err).ToNot(HaveOccurred())
			_, err = io.Copy(os.Stdout, pull)
			Expect(err).ToNot(HaveOccurred())
			defer pull.Close()

			Expect(dockerCli.ImageTag(testsupport.TestContext, tsaTestImage, tsaTargetImageName)).To(Succeed())
			var push io.ReadCloser
			push, err = dockerCli.ImagePush(testsupport.TestContext, tsaTargetImageName, image.PushOptions{RegistryAuth: base64.StdEncoding.EncodeToString([]byte("{}"))})
			Expect(err).ToNot(HaveOccurred())
			_, err = io.Copy(os.Stdout, push)
			Expect(err).ToNot(HaveOccurred())
			defer push.Close()
		} else {
			tsaTargetImageName = api.GetValueFor(api.TargetImageName)
			Expect(tsaTargetImageName).NotTo(BeEmpty(), "TARGET_IMAGE_NAME environment variable must be set when MANUAL_IMAGE_SETUP is true")
		}
	})

	Describe("Cosign initialize", func() {
		It("should initialize the cosign root", func() {
			Expect(cosign.Command(testsupport.TestContext, "initialize").Run()).To(Succeed())
		})
	})

	Describe("cosign sign tsa", func() {
		It("should sign the container using TSA", func() {
			token, err := testsupport.GetOIDCToken(testsupport.TestContext)
			Expect(err).ToNot(HaveOccurred())
			Expect(cosign.Command(testsupport.TestContext, "sign", "-y", "--timestamp-server-url", api.GetValueFor(api.TsaURL), "--identity-token="+token, tsaTargetImageName).Run()).To(Succeed())
		})
	})

	Describe("download tsa chain", func() {
		It("should download the tsa chain", func() {
			tsaChainPath = filepath.Join(tempDir, "ts_chain.pem")

			resp, err := http.Get(api.GetValueFor(api.TsaURL) + "/certchain")
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				logrus.Fatal("failed to download TSA chain")
			}

			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			Expect(os.WriteFile(tsaChainPath, body, 0600)).To(Succeed())
		})
	})

	Describe("cosign verify tsa", func() {
		It("should verify the signature using TSA", func() {
			Expect(cosign.Command(testsupport.TestContext, "verify", "--timestamp-certificate-chain", tsaChainPath, "--certificate-identity-regexp", ".*"+regexp.QuoteMeta(api.GetValueFor(api.OidcUserDomain)), "--certificate-oidc-issuer-regexp", regexp.QuoteMeta(api.GetValueFor(api.OidcIssuerURL)), tsaTargetImageName).Run()).To(Succeed())
		})
	})
})
