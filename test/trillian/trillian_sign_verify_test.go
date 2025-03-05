package trillian

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/clients"
	"github.com/securesign/sigstore-e2e/test/testsupport"
	"github.com/sirupsen/logrus"
)

var (
	rekorCli       *clients.RekorCli
	updateTree     *clients.UpdateTree
	createTree     *clients.CreateTree
	rekorURL       string
	trillianServer string
	currentTreeID  string
	newTreeID      string
	testDir        = "test"
	shardLength    string
	publicKey      string
	err            error
)

var _ = Describe("Trillian tools - CreateTree and UpdateTree", Ordered, func() {
	BeforeAll(func() {
		err = testsupport.CheckMandatoryAPIConfigValues(api.OidcRealm)
		if err != nil {
			Skip("Skip this test - " + err.Error())
		}

		rekorCli = clients.NewRekorCli()
		updateTree = clients.NewUpdateTree()
		createTree = clients.NewCreateTree()

		Expect(testsupport.InstallPrerequisites(
			rekorCli,
			updateTree,
			createTree,
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

	Describe("Start port-forwarding to Trillian server", func() {
		It("should start port-forwarding", func() {
			trillianServer = "trillian-logserver:8091" // port forward to trillian-logserver service
			cmd := exec.Command("oc", "port-forward", "svc/trillian-logserver", "8091:8091")
			cmd.Stdout = GinkgoWriter
			cmd.Stderr = GinkgoWriter
			err = cmd.Start()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Get current tree ID", func() {
		It("should retrieve and validate tree ID", func() {
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "loginfo", "--rekor_server", rekorURL, "--format", "json")
			Expect(err).NotTo(HaveOccurred())

			outputStr := strings.TrimSpace(string(output))
			startIdx := strings.Index(outputStr, "{")

			if startIdx == -1 {
				Fail("No JSON object found in response")
			}

			jsonStr := outputStr[startIdx:]

			if !json.Valid([]byte(jsonStr)) {
				Fail("Extracted JSON is not valid")
			}

			var logInfo map[string]interface{}
			err = json.Unmarshal([]byte(jsonStr), &logInfo)
			Expect(err).NotTo(HaveOccurred())

			fmt.Println("Parsed JSON keys:", logInfo)

			treeIDValue, exists := logInfo["TreeID"]
			Expect(exists).To(BeTrue(), "Key 'TreeID' not found in response")
			currentTreeID = treeIDValue.(string)

			Expect(currentTreeID).NotTo(BeEmpty())
			fmt.Println("Current TreeID:", currentTreeID)
		})
	})

	Describe("Get frozen tree length", func() {
		It("should retrieve tree size", func() {
			output, err := rekorCli.CommandOutput(testsupport.TestContext, "loginfo", "--rekor_server", rekorURL, "--format", "json")
			Expect(err).NotTo(HaveOccurred())

			outputStr := strings.TrimSpace(string(output))

			re := regexp.MustCompile(`\{.*\}`)
			jsonMatch := re.FindString(outputStr)

			if jsonMatch == "" {
				Fail("No JSON object found in response")
			}

			if !json.Valid([]byte(jsonMatch)) {
				Fail("Extracted JSON is not valid")
			}

			var logInfo map[string]interface{}
			err = json.Unmarshal([]byte(jsonMatch), &logInfo)
			Expect(err).NotTo(HaveOccurred())

			fmt.Println("Parsed JSON keys:", logInfo)

			treeSizeValue, exists := logInfo["ActiveTreeSize"]
			Expect(exists).To(BeTrue(), "Key 'ActiveTreeSize' not found in response")

			shardLength = fmt.Sprintf("%.0f", treeSizeValue.(float64))
			Expect(shardLength).NotTo(BeEmpty())

			fmt.Println("Retrieved shard length:", shardLength)
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

	// create new tree for testing
	Describe("Create a new Trillian tree", func() {
		It("Should create a new Trillian tree", func() {
			trillianServer := "localhost:8091"
			output, err := createTree.CommandOutput(testsupport.TestContext, "--admin_server", trillianServer, "tree_state", "ACTIVE", "--max_root_duration", "3600s")
			Expect(err).NotTo(HaveOccurred())
			outputStr := strings.TrimSpace(string(output))
			startIdx := strings.Index(outputStr, "{")

			if startIdx == -1 {
				Fail("No JSON object found in response")
			}
			treeIDStart := strings.Index(outputStr, "Initialised Log (")
			if treeIDStart == -1 {
				Fail("TreeID not found in response")
			}
			treeIDStart += len("Initialised Log (")
			treeIDEnd := strings.Index(outputStr[treeIDStart:], ")")
			if treeIDEnd == -1 {
				Fail("TreeID end not found in response")
			}
			newTreeID = outputStr[treeIDStart : treeIDStart+treeIDEnd]
			fmt.Println("New TreeID:", newTreeID)
			Expect(newTreeID).NotTo(BeEmpty())

		})
	})

	// patching securesign with new tree to manipulate with new tree
	Describe("Generate JSON Patch and Apply OpenShift Patch", func() {
		It("should create a JSON patch and apply it", func() {
			newID, err := strconv.Atoi(newTreeID)
			Expect(err).NotTo(HaveOccurred())
			currTreeID, err := strconv.Atoi(currentTreeID)
			Expect(err).NotTo(HaveOccurred())
			shardL, err := strconv.Atoi(shardLength)
			Expect(err).NotTo(HaveOccurred())
			publicKeyRaw := publicKey
			// normalizing output of key because of BEGIN and END of the key
			publicKeyLines := strings.Split(publicKeyRaw, "\n")
			if len(publicKeyLines) > 2 {
				publicKeyLines = publicKeyLines[1 : len(publicKeyLines)-1]
			}
			publicK := strings.Join(publicKeyLines, "")

			patchData := []map[string]interface{}{
				{
					"op":    "replace",
					"path":  "/spec/rekor/treeID",
					"value": newID,
				},
				{
					"op":   "add",
					"path": "/spec/rekor/sharding/-",
					"value": map[string]interface{}{
						"treeID":           currTreeID,
						"treeLength":       shardL,
						"encodedPublicKey": publicK,
					},
				},
			}

			jsonPatch, err := json.MarshalIndent(patchData, "", "  ")
			Expect(err).NotTo(HaveOccurred())
			testDir, err = os.MkdirTemp("", "tmp")
			Expect(err).NotTo(HaveOccurred())
			patchFilePath := filepath.Join(testDir, "securesign_patch.json")
			err = os.WriteFile(patchFilePath, jsonPatch, 0644)
			Expect(err).NotTo(HaveOccurred())

			fmt.Printf("JSON Patch file created: %s\n", patchFilePath)

			cmd := exec.Command("oc", "patch", "securesign", "securesign-sample", "--type=json", "-p", string(jsonPatch))

			output, err := cmd.CombinedOutput()
			fmt.Println("OpenShift Patch Output:\n", string(output))

			Expect(err).NotTo(HaveOccurred())
			time.Sleep(10 * time.Second) // waiting for the patch to be applied
		})

	})

	// updating the new created tree by frozing it
	Describe("Update the Trillian tree", func() {
		It("Should update the Trillian tree", func() {
			trillianServer := "localhost:8091"
			output, err := updateTree.CommandOutput(testsupport.TestContext, "--admin_server", trillianServer, "--tree_id", newTreeID, "--tree_state", "FROZEN", "--print")
			fmt.Println(newTreeID)
			Expect(err).NotTo(HaveOccurred())
			fmt.Println(output)
		})
	})

	// verify the new tree
	Describe("Verify new tree", func() {
		It("should confirm tree is active", func() {
			Eventually(func() string {
				fmt.Printf("Attempting to get loginfo from: %s\n", rekorURL)

				output, err := rekorCli.CommandOutput(testsupport.TestContext, "loginfo", "--rekor_server", rekorURL)
				if err != nil {
					fmt.Printf("Error running loginfo command: %v\n", err)
					return ""
				}
				fmt.Printf("loginfo output: %s\n", output)
				lines := strings.Split(string(output), "\n")
				for _, line := range lines {
					if strings.Contains(line, "TreeID:") {
						parts := strings.Split(line, ":")
						if len(parts) > 1 {
							treeID := strings.TrimSpace(parts[1])
							return treeID
						}
					}
				}

				fmt.Println("Could not extract TreeID from output")
				return ""
			}, "30s", "10s").Should(Equal(newTreeID), "TreeID does not match expected value")
		})
	})

})
var _ = AfterSuite(func() {
	// Cleanup shared resources after all tests have run.
	pkillCmd := exec.Command("pkill", "-f", "oc port-forward")
	pkillCmd.Run()

	Expect(os.RemoveAll(testDir)).To(Succeed())
})
