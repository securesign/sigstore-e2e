package cosign

import (
	"fmt"
	"io"
	"os"
	"sigstore-e2e-test/pkg/api"
	"sigstore-e2e-test/pkg/clients"
	"sigstore-e2e-test/test/testsupport"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

const testImage string = "alpine:latest"

var cli *client.Client

var _ = Describe("Cosign test", Ordered, func() {

	fmt.Println(api.Values.GetString(api.FulcioURL))

	var err error
	var cosign = clients.NewCosign()

	targetImageName := "ttl.sh/" + uuid.New().String() + ":5m"
	BeforeAll(func() {

		Expect(testsupport.InstallPrerequisites(
			cosign,
		)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

		cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		Expect(err).ToNot(HaveOccurred())

		var pull io.ReadCloser
		pull, err = cli.ImagePull(testsupport.TestContext, testImage, types.ImagePullOptions{})
		Expect(err).ToNot(HaveOccurred())
		_, err = io.Copy(os.Stdout, pull)
		Expect(err).ToNot(HaveOccurred())
		defer pull.Close()

		Expect(cli.ImageTag(testsupport.TestContext, testImage, targetImageName)).To(Succeed())
		var push io.ReadCloser
		push, err = cli.ImagePush(testsupport.TestContext, targetImageName, types.ImagePushOptions{RegistryAuth: types.RegistryAuthFromSpec})
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
				"--mirror="+api.Values.GetString(api.TufURL),
				"--root="+api.Values.GetString(api.TufURL)+"/root.json").Run()).To(Succeed())
		})
	})

	Describe("cosign sign", func() {
		It("should sign the container", func() {
			token, err := testsupport.GetOIDCToken(testsupport.TestContext, api.Values.GetString(api.OidcIssuerURL), "jdoe", "secure", api.Values.GetString(api.OidcRealm))
			Expect(err).ToNot(HaveOccurred())
			Expect(cosign.Command(testsupport.TestContext, "sign",
				"-y", "--fulcio-url="+api.Values.GetString(api.FulcioURL), "--rekor-url="+api.Values.GetString(api.RekorURL), "--oidc-issuer="+api.Values.GetString(api.OidcIssuerURL), "--identity-token="+token, targetImageName).Run()).To(Succeed())
		})
	})

	Describe("cosign verify", func() {
		It("should verify the signature", func() {
			Expect(cosign.Command(testsupport.TestContext, "verify", "--rekor-url="+api.Values.GetString(api.RekorURL), "--certificate-identity-regexp", ".*@redhat", "--certificate-oidc-issuer-regexp", ".*keycloak.*", targetImageName).Run()).To(Succeed())
		})
	})
})
