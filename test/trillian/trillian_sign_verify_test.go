package trillian

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	rekorCli      *clients.RekorCli
	rekorURL      string
	currentTreeID string
	newTreeID     string
	newID         string
	testDir       string
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

			outputStr := strings.TrimSpace(string(output))
			startIdx := strings.LastIndex(outputStr, "{")

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

	Describe("Set tree state to DRAINING", func() {
		It("should update tree state", func() {
			cmd := fmt.Sprintf("oc run --image registry.redhat.io/rhtas/updatetree-rhel9@sha256:1a95a2061b9bc0613087903425d84024ce10e00bc6110303a75637fb15d95d34 --restart=Never --attach=true --rm=true -q -- updatetree --admin_server=trillian-logserver:8091 --tree_id=%s --tree_state=DRAINING", currentTreeID)
			out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
			if err != nil {
				fmt.Printf("Command failed with error: %v\n", err)
			}
			fmt.Printf("Command output: %s\n", string(out))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Set tree state to FROZEN", func() {
		It("should freeze the tree", func() {
			freezeCmd := exec.Command("oc", "run",
				"--image", "registry.redhat.io/rhtas/updatetree-rhel9@sha256:1a95a2061b9bc0613087903425d84024ce10e00bc6110303a75637fb15d95d34",
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

	Describe("Create new Merkle tree", func() {
		It("should create tree", func() {
			createCmd := exec.Command("oc", "run",
				"createtree",
				"--image", "registry.redhat.io/rhtas/createtree-rhel9@sha256:f66a707e68fb0cdcfcddc318407fe60d72f50a7b605b5db55743eccc14a422ba",
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
			fmt.Print("new TreeID: " + newTreeID + "\n")
		})
	})

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
		})
	})

	Describe("Verify new tree", func() {
		It("should confirm tree is active", func() {
			Eventually(func() bool {
				// rekor server pod has not been available after createTree for some reason
				fmt.Println("Checking Rekor server availability...")

				resp, err := http.Get(rekorURL + "/api/v1/log")
				if err != nil {
					fmt.Println("Rekor server not ready yet, retrying...")
					return false
				}
				defer resp.Body.Close()

				return resp.StatusCode == http.StatusOK
			}, 3*time.Minute, 10*time.Second).Should(BeTrue(), "Rekor server did not become available")
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
			}, "1m", "10s").Should(Equal(newID)) // making sure everything is ready and waiting
		})
	})

})

var _ = AfterSuite(func() {
	// Cleanup shared resources after all tests have run.
	Expect(os.RemoveAll(testDir)).To(Succeed())
})
