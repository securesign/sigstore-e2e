package tuftool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/securesign/sigstore-e2e/pkg/clients"
	"github.com/securesign/sigstore-e2e/test/testsupport"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var workdir string
var root string
var input string
var keyDir string
var tufRepo string
var rootDir string

const ctlogPubkeyPath string = "targets/ctfe.pub"
const fulcioCertPath string = "targets/fulcio_v1.crt.pem"
const rekorPubkeyPath string = "targets/rekor.pub"
const tsaCertPath string = "targets/tsa.certchain.pem"

const expirationWeeks52 string = "in 52 weeks"
const expirationDays10 string = "in 10 days"

var _ = Describe("TUF manual repo test", Ordered, func() {

	var (
		err     error
		tuftool *clients.Tuftool
	)

	BeforeAll(func() {

		tuftool = clients.NewTuftool()

		Expect(testsupport.InstallPrerequisites(tuftool)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

		workdir, err = os.MkdirTemp("", "trustroot_example")
		Expect(err).ToNot(HaveOccurred())

		logrus.Infof("Created temporary directory: %s", workdir)
	})

	It("should setup repository via tuftool", func() {
		setupManualTufRepo(tuftool)
	})

	It("should verify workdir structure", func() {
		verifyWorkdirStructure(workdir)
	})
})

func setupManualTufRepo(tuftool *clients.Tuftool) {
	// "Tuf repo directory"
	root = filepath.Join(workdir, "root", "root.json")
	rootDir = filepath.Join(workdir, "root")
	input = filepath.Join(workdir, "input")
	keyDir = filepath.Join(workdir, "keys")
	tufRepo = filepath.Join(workdir, "tuf-repo")

	err := os.MkdirAll(rootDir, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())
	err = os.MkdirAll(keyDir, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())
	err = os.MkdirAll(input, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())
	err = os.MkdirAll(tufRepo, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	Expect(tuftool.Command(testsupport.TestContext, "root", "init", root).Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "expire", root, expirationWeeks52).Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "root", "set-threshold", root, "root", "1").Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "set-threshold", root, "snapshot", "1").Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "set-threshold", root, "targets", "1").Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "set-threshold", root, "timestamp", "1").Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "root", "gen-rsa-key", root, keyDir+"/root.pem", "--role", "root").Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "gen-rsa-key", root, keyDir+"/snapshot.pem", "--role", "snapshot").Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "gen-rsa-key", root, keyDir+"/targets.pem", "--role", "targets").Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "gen-rsa-key", root, keyDir+"/timestamp.pem", "--role", "timestamp").Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "root", "sign", root, "-k", keyDir+"/root.pem").Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "create",
		"--root", root,
		"--key", keyDir+"/root.pem",
		"--key", keyDir+"/snapshot.pem",
		"--key", keyDir+"/targets.pem",
		"--key", keyDir+"/timestamp.pem",
		"--add-targets", input,
		"--targets-expires", expirationWeeks52,
		"--targets-version", "1",
		"--snapshot-expires", expirationWeeks52,
		"--snapshot-version", "1",
		"--timestamp-expires", expirationDays10,
		"--timestamp-version", "1",
		"--outdir", tufRepo).Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "rhtas",
		"--root", root,
		"--key", keyDir+"/root.pem",
		"--key", keyDir+"/snapshot.pem",
		"--key", keyDir+"/targets.pem",
		"--key", keyDir+"/timestamp.pem",
		"--set-ctlog-target", ctlogPubkeyPath,
		"--ctlog-uri", "https://ctlog.rhtas",
		"--targets-expires", expirationWeeks52,
		"--targets-version", "1",
		"--snapshot-expires", expirationWeeks52,
		"--snapshot-version", "1",
		"--timestamp-expires", expirationDays10,
		"--timestamp-version", "1",
		"--force-version",
		"--outdir", tufRepo,
		"--metadata-url", "file://"+tufRepo).Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "rhtas",
		"--root", root,
		"--key", keyDir+"/root.pem",
		"--key", keyDir+"/snapshot.pem",
		"--key", keyDir+"/targets.pem",
		"--key", keyDir+"/timestamp.pem",
		"--set-fulcio-target", fulcioCertPath,
		"--fulcio-uri", "https://fulcio.rhtas",
		"--targets-expires", expirationWeeks52,
		"--targets-version", "1",
		"--snapshot-expires", expirationWeeks52,
		"--snapshot-version", "1",
		"--timestamp-expires", expirationDays10,
		"--timestamp-version", "1",
		"--force-version",
		"--outdir", tufRepo,
		"--metadata-url", "file://"+tufRepo).Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "rhtas",
		"--root", root,
		"--key", keyDir+"/root.pem",
		"--key", keyDir+"/snapshot.pem",
		"--key", keyDir+"/targets.pem",
		"--key", keyDir+"/timestamp.pem",
		"--set-rekor-target", rekorPubkeyPath,
		"--rekor-uri", "https://rekor.rhtas",
		"--targets-expires", expirationWeeks52,
		"--targets-version", "1",
		"--snapshot-expires", expirationWeeks52,
		"--snapshot-version", "1",
		"--timestamp-expires", expirationDays10,
		"--timestamp-version", "1",
		"--force-version",
		"--outdir", tufRepo,
		"--metadata-url", "file://"+tufRepo).Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "rhtas",
		"--root", root,
		"--key", keyDir+"/root.pem",
		"--key", keyDir+"/snapshot.pem",
		"--key", keyDir+"/targets.pem",
		"--key", keyDir+"/timestamp.pem",
		"--set-tsa-target", tsaCertPath,
		"--tsa-uri", "https://tsa.rhtas",
		"--targets-expires", expirationWeeks52,
		"--targets-version", "1",
		"--snapshot-expires", expirationWeeks52,
		"--snapshot-version", "1",
		"--timestamp-expires", expirationDays10,
		"--timestamp-version", "1",
		"--force-version",
		"--outdir", tufRepo,
		"--metadata-url", "file://"+tufRepo).Run()).To(Succeed())
}

func verifyWorkdirStructure(rootPath string) {
	expectedDirs := map[string]bool{
		"input":            true,
		"keys":             true,
		"root":             true,
		"tuf-repo":         true,
		"tuf-repo/targets": true,
	}

	expectedFiles := map[string]bool{
		"keys/root.pem":            true,
		"keys/snapshot.pem":        true,
		"keys/targets.pem":         true,
		"keys/timestamp.pem":       true,
		"root/root.json":           true,
		"tuf-repo/1.root.json":     true,
		"tuf-repo/1.snapshot.json": true,
		"tuf-repo/1.targets.json":  true,
		"tuf-repo/root.json":       true,
		"tuf-repo/timestamp.json":  true,
	}

	targetSuffixes := []string{
		".tsa.certchain.pem",
		".trusted_root.json",
		".fulcio_v1.crt.pem",
		".ctfe.pub",
		".rekor.pub",
	}

	foundSuffixesCount := make(map[string]int)
	for _, suffix := range targetSuffixes {
		foundSuffixesCount[suffix] = 0 // Initialize count for each suffix
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		Expect(err).ToNot(HaveOccurred())

		relPath, err := filepath.Rel(rootPath, path)
		Expect(err).ToNot(HaveOccurred())

		// skip root directory
		if relPath == "." {
			return nil
		}

		if info.IsDir() {
			_, exists := expectedDirs[relPath]
			Expect(exists).To(BeTrue(), "unexpected directory: "+filepath.Join(rootPath, relPath))
			delete(expectedDirs, relPath)
		} else {
			if strings.HasPrefix(relPath, "tuf-repo/targets/") {
				validSuffix := false
				for _, suffix := range targetSuffixes {
					if strings.HasSuffix(relPath, suffix) {
						foundSuffixesCount[suffix]++
						validSuffix = true
						break
					}
				}
				Expect(validSuffix).To(BeTrue(), "unexpected file in targets: "+filepath.Join(rootPath, relPath))
			} else {
				_, exists := expectedFiles[relPath]
				Expect(exists).To(BeTrue(), "unexpected file: "+filepath.Join(rootPath, relPath))
				delete(expectedFiles, relPath)
			}
		}

		return nil
	})

	Expect(err).ToNot(HaveOccurred())

	if len(expectedDirs) > 0 {
		missingDirs := []string{}
		for dir := range expectedDirs {
			missingDirs = append(missingDirs, filepath.Join(workdir, dir))
		}
		Expect(missingDirs).To(BeEmpty(), fmt.Sprintf("missing directories: %v", missingDirs))
	}

	if len(expectedFiles) > 0 {
		missingFiles := []string{}
		for file := range expectedFiles {
			missingFiles = append(missingFiles, filepath.Join(workdir, file))
		}
		Expect(missingFiles).To(BeEmpty(), fmt.Sprintf("missing files: %v", missingFiles))
	}

	for suffix, count := range foundSuffixesCount {
		if suffix == ".trusted_root.json" {
			// Allow multiple .trusted_root.json files
			Expect(count).To(BeNumerically(">=", 1), fmt.Sprintf("Expected at least one .trusted_root.json file, found %d", count))
		} else {
			// Ensure only one file for other suffixes
			Expect(count).To(Equal(1), fmt.Sprintf("Expected exactly one file with suffix %s, found %d", suffix, count))
		}
	}
}

var _ = AfterSuite(func() {
	// Cleanup shared resources after all tests have run.
	Expect(os.RemoveAll(workdir)).To(Succeed())
})
