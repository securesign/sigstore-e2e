package cosign

import (
	"regexp"

	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/clients"
	"github.com/securesign/sigstore-e2e/test/testsupport"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var tsaTargetImageName string

var _ = Describe("TSA test", Ordered, func() {

	var (
		err    error
		cosign *clients.Cosign
	)

	BeforeAll(func() {
		logrus.Infof("Starting TSA cosign test")
		err = testsupport.CheckMandatoryAPIConfigValues(api.OidcRealm)
		if err != nil {
			Fail(err.Error())
		}

		cosign = clients.NewCosign()

		Expect(testsupport.InstallPrerequisites(cosign)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

		manualImageSetup := api.GetValueFor(api.ManualImageSetup) == "true"
		if !manualImageSetup {
			tsaTargetImageName, err = testsupport.PushTestImage(testsupport.TestContext)
			Expect(err).ToNot(HaveOccurred())
		} else {
			tsaTargetImageName = api.GetValueFor(api.TargetImageName)
			Expect(tsaTargetImageName).NotTo(BeEmpty(), "TARGET_IMAGE_NAME environment variable must be set when MANUAL_IMAGE_SETUP is true")
		}
	})

	Describe("Cosign initialize", func() {
		It("should initialize the cosign root", func() {
			Eventually(func() error {
				return cosign.Command(testsupport.TestContext, "initialize").Run()
			}).WithTimeout(testsupport.CommandRetryTimeout).WithPolling(testsupport.CommandRetryInterval).Should(Succeed())
		})
	})

	Describe("cosign sign tsa", func() {
		It("should sign the container using TSA", func() {
			token, err := testsupport.GetOIDCToken(testsupport.TestContext)
			Expect(err).ToNot(HaveOccurred())
			Expect(cosign.Command(testsupport.TestContext, "sign", "--identity-token="+token, tsaTargetImageName).Run()).To(Succeed())
		})
	})

	Describe("cosign verify tsa", func() {
		It("should verify the signature using TSA", func() {
			Eventually(func() error {
				return cosign.Command(testsupport.TestContext, "verify", "--use-signed-timestamps", "--certificate-identity-regexp", ".*"+regexp.QuoteMeta(api.GetValueFor(api.OidcUserDomain)), "--certificate-oidc-issuer-regexp", regexp.QuoteMeta(api.GetValueFor(api.OidcIssuerURL)), tsaTargetImageName).Run()
			}).WithTimeout(testsupport.CommandRetryTimeout).WithPolling(testsupport.CommandRetryInterval).Should(Succeed())
		})
	})
})
