package trillian

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCosignTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Trillian Tree")
}
