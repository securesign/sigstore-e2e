package gitsign

import (
	"os"
	"sigstore-e2e-test/pkg/tas"
	"sigstore-e2e-test/pkg/tas/gitsign"
	"sigstore-e2e-test/pkg/tekton"
	"sigstore-e2e-test/test/testSupport"
	"testing"
)

func TestMain(m *testing.M) {
	if err := testSupport.InstallPrerequisites(
		tekton.NewTektonInstaller(testSupport.TestContext),
		tas.NewTas(testSupport.TestContext),
		gitsign.NewGitsignInstaller(testSupport.TestContext),
	); err != nil {
		panic(err)
	}
	status := m.Run()
	testSupport.DestroyPrerequisites()
	os.Exit(status)
}
