package gitsign

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigstore-e2e-test/pkg/tas"
	"sigstore-e2e-test/pkg/tas/gitsign"
	"sigstore-e2e-test/pkg/tekton"
	"sigstore-e2e-test/test/testSupport"
)

var _ = BeforeSuite(func() {
	Expect(testSupport.InstallPrerequisites(
		tekton.NewTektonInstaller(testSupport.TestContext),
		tas.NewTas(testSupport.TestContext),
		gitsign.NewGitsignInstaller(testSupport.TestContext),
	)).To(Succeed())
})

var _ = AfterSuite(func() {
	testSupport.DestroyPrerequisites()
})
