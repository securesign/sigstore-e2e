package trillian

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/clients"
	"github.com/securesign/sigstore-e2e/test/testsupport"
	"github.com/sirupsen/logrus"
)

var (
	updateTree *clients.UpdateTree
	createTree *clients.CreateTree
	err        error
)

var _ = Describe("Trillian tools - CreateTree and UpdateTree", Ordered, func() {
	BeforeAll(func() {
		err = testsupport.CheckMandatoryAPIConfigValues(api.OidcRealm)
		if err != nil {
			Skip("Skip this test - " + err.Error())
		}

		updateTree = clients.NewUpdateTree()
		createTree = clients.NewCreateTree()

		Expect(testsupport.InstallPrerequisites(
			updateTree,
			createTree,
		)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})
	})

	Describe("Execute createtree help", func() {
		It("run help command", func() {

			output, err := createTree.CommandOutput(testsupport.TestContext, "--help")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("Usage of ")) // just to make sure output appeared
		})
	})

	Describe("Execute updatetree help", func() {
		It("run help command", func() {

			output, err := updateTree.CommandOutput(testsupport.TestContext, "--help")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("Usage of ")) // just to make sure output appeared
		})
	})
})
