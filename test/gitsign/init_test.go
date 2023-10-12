package gitsign

import (
	"os"
	"sigstore-e2e-test/pkg/tekton"
	"sigstore-e2e-test/test/support"
	"testing"
)

func TestMain(m *testing.M) {
	if err := support.InstallPrerequisites(tekton.New(support.TestContext)); err != nil {
		panic(err)
	}
	defer func() {
		// the defer does not work after panic coming from the test
		if err := support.DestroyPrerequisities(); err != nil {
			panic(err)
		}
	}()
	status := m.Run()
	os.Exit(status)
}
