package cosign

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"sigstore-e2e-test/pkg/tas"
	"sigstore-e2e-test/pkg/tas/cosign"
	"sigstore-e2e-test/test/testSupport"
	"testing"
	"time"
)

var cli *client.Client

func TestCosignSignVerify(t *testing.T) {
	RegisterTestingT(t)
	var err error
	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	Expect(err).To(BeNil())
	targetImageName := "ttl.sh/" + uuid.New().String() + ":5m"
	Expect(cli.ImageTag(testSupport.TestContext, "alpine:latest", targetImageName)).To(Succeed())
	_, err = cli.ImagePush(testSupport.TestContext, targetImageName, types.ImagePushOptions{RegistryAuth: types.RegistryAuthFromSpec})
	Expect(err).To(BeNil())
	// wait for a while to be sure that the image is pushed
	time.Sleep(30 * time.Second)

	t.Run("cosign initialize", func(t *testing.T) {
		Expect(cosign.Cosign(testSupport.TestContext,
			"initialize",
			"--mirror="+tas.TufURL,
			"--root="+tas.TufURL+"/root.json")).To(Succeed())
	})

	t.Run("cosign sign", func(t *testing.T) {
		token, err := testSupport.GetOIDCToken(tas.OidcIssuerURL, "jdoe", "secure", tas.OIDC_REALM)
		Expect(err).To(BeNil())
		Expect(cosign.Cosign(testSupport.TestContext,
			"sign", "-y", "--fulcio-url="+tas.FulcioURL, "--rekor-url="+tas.RekorURL, "--oidc-issuer="+tas.OidcIssuerURL, "--identity-token="+token, targetImageName)).To(Succeed())
	})

	t.Run("cosign verify", func(t *testing.T) {
		Expect(cosign.Cosign(testSupport.TestContext, "verify", "--rekor-url="+tas.RekorURL, "--certificate-identity-regexp", ".*@redhat", "--certificate-oidc-issuer-regexp", ".*keycloak.*", targetImageName)).To(Succeed())
	})

}
