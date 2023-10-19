package cosign

import (
	"github.com/sirupsen/logrus"
	"os"
	"sigstore-e2e-test/pkg/tas"
	"sigstore-e2e-test/pkg/tas/cosign"
	"sigstore-e2e-test/test/testSupport"
	"testing"
)

func TestMain(m *testing.M) {
	logrus.SetLevel(logrus.DebugLevel)
	if err := testSupport.InstallPrerequisites(
		tas.NewTas(testSupport.TestContext),
		cosign.NewCosign(testSupport.TestContext),
	); err != nil {
		panic(err)
	}
	status := m.Run()
	testSupport.DestroyPrerequisites()
	os.Exit(status)
}
