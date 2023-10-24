package cosign

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigstore-e2e-test/pkg/tas"
	"sigstore-e2e-test/pkg/tas/cosign"
	"sigstore-e2e-test/test/testSupport"
)

var _ = BeforeSuite(func() {
	Expect(testSupport.InstallPrerequisites(
		tas.NewTas(testSupport.TestContext),
		cosign.NewCosign(testSupport.TestContext),
	)).To(Succeed())
})

var _ = AfterSuite(func() {
	testSupport.DestroyPrerequisites()
})
