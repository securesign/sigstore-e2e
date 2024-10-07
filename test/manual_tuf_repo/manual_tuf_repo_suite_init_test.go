package manualtuf

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestManualTUFRepoTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Create tuf repo manually")
}
