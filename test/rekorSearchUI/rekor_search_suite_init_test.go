package rekorSearchUI

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRekorSearchUIE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Search entries in UI")
}
