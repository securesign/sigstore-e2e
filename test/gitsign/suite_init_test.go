package gitsign

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGitsignE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sign commit with gitsign tool")
}
