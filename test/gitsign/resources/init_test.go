package gitsign

import (
	"os"
	"sigstore-e2e-test/pkg/tas"
	"sigstore-e2e-test/pkg/tekton"
	"sigstore-e2e-test/test/testSupport"
	"testing"
)

func TestMain(m *testing.M) {
	if err := testSupport.InstallPrerequisites(
		tekton.NewTektonInstaller(testSupport.TestContext),
		tas.NewTas(testSupport.TestContext),
	); err != nil {
		panic(err)
	}
	defer func() {
		// the defer does not work after panic coming from the test
		if err := testSupport.DestroyPrerequisites(); err != nil {
			panic(err)
		}
	}()
	status := m.Run()
	os.Exit(status)
}
