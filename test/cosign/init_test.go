package cosign

import (
	"os"
	"sigstore-e2e-test/pkg/tas"
	"sigstore-e2e-test/pkg/tas/cosign"
	"sigstore-e2e-test/test/testSupport"
	"testing"
)

func TestMain(m *testing.M) {
	if err := testSupport.InstallPrerequisites(
		tas.NewTas(testSupport.TestContext),
		cosign.NewCosign(testSupport.TestContext),
	); err != nil {
		panic(err)
	}
	status := m.Run()
	testSupport.DestroyPrerequisities()
	os.Exit(status)
}
