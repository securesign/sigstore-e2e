package rekorcli

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCosignTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Make and verify entries, query the log for inclusion proof, integrity verification of the log or retrieval of entries")
}
