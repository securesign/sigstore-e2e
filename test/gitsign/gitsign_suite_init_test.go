package gitsign

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGitSignE2eTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sign commigs with gitsign tool")
}
