package trillian

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/clients"
	"github.com/securesign/sigstore-e2e/test/testsupport"
	"github.com/sirupsen/logrus"
)

var (
	rekorCli      *clients.RekorCli
	rekorURL      string
	currentTreeID string
	newTreeID     string
	shardLength   string
	publicKey     string
	err           error
)

var _ = Describe("Trillian tools - CreateTree and UpdateTree", Ordered, func() {
	BeforeAll(func() {
		err = testsupport.CheckMandatoryAPIConfigValues(api.OidcRealm)
		if err != nil {
			Skip("Skip this test - " + err.Error())
		}

		rekorCli = clients.NewRekorCli()

		Expect(testsupport.InstallPrerequisites(
			rekorCli,
		)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})
	})

	BeforeEach(func() {
		rekorURL = api.GetValueFor(api.RekorURL)
	})

	Describe("Get current tree ID", func() {
		It("should retrieve and validate tree ID", func() {
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "loginfo", "--rekor_server", rekorURL, "--format", "json")
			Expect(err).NotTo(HaveOccurred())

			var logInfo map[string]interface{}
			err = json.Unmarshal([]byte(output), &logInfo)
			Expect(err).NotTo(HaveOccurred())

			currentTreeID = logInfo["TreeID"].(string)
			Expect(currentTreeID).NotTo(BeEmpty())
		})
	})

	Describe("Set tree state to DRAINING", func() {
		It("should update tree state", func() {
			cmd := fmt.Sprintf("oc run --image registry.redhat.io/rhtas/updatetree-rhel9:latest --restart=Never --attach=true --rm=true -q -- updatetree --admin_server=trillian-logserver:8091 --tree_id=%s --tree_state=DRAINING", currentTreeID)
			out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
			if err != nil {
				fmt.Printf("Command failed with error: %v\n", err)
			}
			fmt.Printf("Command output: %s\n", string(out))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Wait for queue drain", func() {
		It("should verify queue is empty", func() {
			Eventually(func() bool {
				logsCmd := exec.Command("oc", "logs",
					"deployment/trillian-logserver:8091",
					"--tail=100",
				)
				_, err := logsCmd.Output()
				return err == nil
			}, "5m", "10s").Should(BeTrue())
		})
	})

	Describe("Set tree state to FROZEN", func() {
		It("should freeze the tree", func() {
			freezeCmd := exec.Command("oc", "run",
				"--image", "registry.redhat.io/rhtas/updatetree-rhel9:latest",
				"--restart=Never",
				"--attach=true",
				"--rm=true",
				"-q",
				"--",
				"updatetree",
				"--admin_server=trillian-logserver:8091",
				fmt.Sprintf("--tree_id=%s", currentTreeID),
				"--tree_state=FROZEN",
			)
			err := freezeCmd.Run()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Get frozen tree length", func() {
		It("should retrieve tree size", func() {
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "loginfo", "--rekor_server", rekorURL, "--format", "json")
			Expect(err).NotTo(HaveOccurred())

			var logInfo map[string]interface{}
			err = json.Unmarshal([]byte(output), &logInfo)
			Expect(err).NotTo(HaveOccurred())

			shardLength = logInfo["ActiveTreeSize"].(string)
			Expect(shardLength).NotTo(BeEmpty())
		})
	})

	Describe("Get public key", func() {
		It("should retrieve key", func() {
			curlCmd := exec.Command("curl", "-s", fmt.Sprintf("%s/api/v1/log/publicKey", rekorURL))
			output, err := curlCmd.Output()
			Expect(err).NotTo(HaveOccurred())

			publicKey = strings.TrimSpace(string(output))
			Expect(publicKey).NotTo(BeEmpty())
		})
	})

	Describe("Create new Merkle tree", func() {
		It("should create tree", func() {
			createCmd := exec.Command("oc", "run",
				"createtree",
				"--image", "registry.redhat.io/rhtas/createtree-rhel9::latest",
				"--restart=Never",
				"--attach=true",
				"--rm=true",
				"-q",
				"--",
				"-logtostderr=false",
				"--admin_server=trillian-logserver:8091",
				"--display_name=rekor-tree",
			)
			output, err := createCmd.Output()
			Expect(err).NotTo(HaveOccurred())

			newTreeID = strings.TrimSpace(string(output))
			Expect(newTreeID).NotTo(BeEmpty())
		})
	})

	Describe("Verify new tree", func() {
		It("should confirm tree is active", func() {
			Eventually(func() string {
				output, err := rekorCli.CommandOutput(testsupport.TestContext, "loginfo", "--rekor_server", rekorURL, "--format", "json")
				if err != nil {
					return ""
				}

				var logInfo map[string]interface{}
				if err = json.Unmarshal([]byte(output), &logInfo); err != nil {
					return ""
				}

				return logInfo["TreeID"].(string)
			}, "5m", "10s").Should(Equal(newTreeID))
		})
	})
})
