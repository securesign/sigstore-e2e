package cosign

import (
	"io"
	"os"
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

var _ = Describe("Cosign test", Ordered, func() {

	var (
		err       error
		dockerCli *client.Client
		cosign    *clients.Cosign
	)
	targetImageName := "ttl.sh/" + uuid.New().String() + ":5m"

	BeforeAll(func() {
		err = testsupport.CheckAPIConfigValues(testsupport.Mandatory, api.FulcioURL, api.RekorURL, api.TufURL, api.OidcIssuerURL, api.OidcRealm)
		if err != nil {
			Skip("Skip this test - " + err.Error())
		}

		cosign = clients.NewCosign()

		Expect(testsupport.InstallPrerequisites(
			cosign,
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
		push, err = dockerCli.ImagePush(testsupport.TestContext, targetImageName, types.ImagePushOptions{RegistryAuth: types.RegistryAuthFromSpec})
		Expect(err).ToNot(HaveOccurred())
		_, err = io.Copy(os.Stdout, push)
		Expect(err).ToNot(HaveOccurred())
		defer push.Close()
		// wait for a while to be sure that the image landed in the registry
		time.Sleep(10 * time.Second)
	})

	Describe("Cosign initialize", func() {
		It("should initialize the cosign root", func() {
			Expect(cosign.Command(testsupport.TestContext, "initialize",
				"--mirror="+api.GetValueFor(api.TufURL),
				"--root="+api.GetValueFor(api.TufURL)+"/root.json").Run()).To(Succeed())
		})
	})

	Describe("cosign sign", func() {
		It("should sign the container", func() {
			token, err := testsupport.GetOIDCToken(testsupport.TestContext, api.GetValueFor(api.OidcIssuerURL), "jdoe", "secure", api.GetValueFor(api.OidcRealm))
			Expect(err).ToNot(HaveOccurred())
			Expect(cosign.Command(testsupport.TestContext, "sign",
				"-y", "--fulcio-url="+api.GetValueFor(api.FulcioURL), "--rekor-url="+api.GetValueFor(api.RekorURL), "--oidc-issuer="+api.GetValueFor(api.OidcIssuerURL), "--identity-token="+token, targetImageName).Run()).To(Succeed())
		})
	})

	Describe("cosign verify", func() {
		It("should verify the signature", func() {
			Expect(cosign.Command(testsupport.TestContext, "verify", "--rekor-url="+api.GetValueFor(api.RekorURL), "--certificate-identity-regexp", ".*@redhat", "--certificate-oidc-issuer-regexp", ".*keycloak.*", targetImageName).Run()).To(Succeed())
		})
	})
})
