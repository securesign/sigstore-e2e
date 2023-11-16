package cosign

import (
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io"
	"os"
	"sigstore-e2e-test/pkg/api"
	"sigstore-e2e-test/pkg/clients"
	"sigstore-e2e-test/test/testSupport"
	"time"
)

const testImage string = "alpine:latest"

var cli *client.Client

var _ = Describe("Cosign test", Ordered, func() {

	fmt.Println(api.Values.GetString(api.FulcioURL))

	var err error
	var cosign = clients.NewCosign(testSupport.TestContext)

	targetImageName := "ttl.sh/" + uuid.New().String() + ":5m"
	BeforeAll(func() {

		Expect(testSupport.InstallPrerequisites(
			cosign,
		)).To(Succeed())

		DeferCleanup(func() { testSupport.DestroyPrerequisites() })

		cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		Expect(err).To(BeNil())

		var pull io.ReadCloser
		pull, err = cli.ImagePull(testSupport.TestContext, testImage, types.ImagePullOptions{})
		io.Copy(os.Stdout, pull)
		defer pull.Close()

		Expect(cli.ImageTag(testSupport.TestContext, testImage, targetImageName)).To(Succeed())
		var push io.ReadCloser
		push, err = cli.ImagePush(testSupport.TestContext, targetImageName, types.ImagePushOptions{RegistryAuth: types.RegistryAuthFromSpec})
		io.Copy(os.Stdout, push)
		defer push.Close()
		Expect(err).To(BeNil())
		// wait for a while to be sure that the image landed in the registry
		time.Sleep(10 * time.Second)
	})

	Describe("Cosign initialize", func() {
		It("should initialize the cosign root", func() {
			Expect(cosign.Command("initialize",
				"--mirror="+api.Values.GetString(api.TufURL),
				"--root="+api.Values.GetString(api.TufURL)+"/root.json").Run()).To(Succeed())
		})
	})

	Describe("cosign sign", func() {
		It("should sign the container", func() {
			token, err := testSupport.GetOIDCToken(api.Values.GetString(api.OidcIssuerURL), "jdoe", "secure", api.Values.GetString(api.OidcRealm))
			Expect(err).To(BeNil())
			Expect(cosign.Command("sign",
				"-y", "--fulcio-url="+api.Values.GetString(api.FulcioURL), "--rekor-url="+api.Values.GetString(api.RekorURL), "--oidc-issuer="+api.Values.GetString(api.OidcIssuerURL), "--identity-token="+token, targetImageName).Run()).To(Succeed())
		})
	})

	Describe("cosign verify", func() {
		It("should verify the signature", func() {
			Expect(cosign.Command("verify", "--rekor-url="+api.Values.GetString(api.RekorURL), "--certificate-identity-regexp", ".*@redhat", "--certificate-oidc-issuer-regexp", ".*keycloak.*", targetImageName).Run()).To(Succeed())
		})
	})
})
