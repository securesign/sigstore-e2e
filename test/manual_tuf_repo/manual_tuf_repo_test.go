package manualtuf

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/securesign/sigstore-e2e/pkg/api"
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
const securesignPath string = "manifests/securesign.yaml"
const pvcPath string = "manifests/pvc.yaml"
const envVariablesPath string = "../../tas-env-variables.sh"

const rootExpiration string = "in 52 weeks"
const snapshotExpiration string = "in 52 weeks"
const targetsExpiration string = "in 52 weeks"
const timestampExpiration string = "in 10 days"

const maxAttempts int = 10

var _ = Describe("TUF manual repo test", Ordered, func() {

	var (
		err     error
		oc      *clients.Oc
		cosign  *clients.Cosign
		tuftool *clients.Tuftool
	)

	BeforeAll(func() {

		logrus.Info("SIGSTORE_OIDC_ISSUER=", api.GetValueFor(api.OidcIssuerURL))
		oc = clients.NewOc()

		cosign = clients.NewCosign()

		tuftool = clients.NewTuftool()

		Expect(testsupport.InstallPrerequisites(oc, cosign, tuftool)).To(Succeed())

		DeferCleanup(func() {
			if err := testsupport.DestroyPrerequisites(); err != nil {
				logrus.Warn("Env was not cleaned-up" + err.Error())
			}
		})

		workdir, err = os.MkdirTemp("", "trustroot_example")
		Expect(err).ToNot(HaveOccurred())

	})

	Describe("Setup TUF repository via tuftool", func() {
		It("should setup maunal TUF repo", func() {
			setupManualTufRepo(tuftool)
		})
	})

	Describe("Setup Securesign Deployment ", func() {
		It("should deploy securesign", func() {
			setupSecuresignDeployment(oc)
		})
	})

	Describe("Wait for Securesign Deployment", func() {
		It("should wait till all securesign instances are ready", func() {
			GetMandatoryAPIConfigValues(maxAttempts)
		})
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
	Expect(tuftool.Command(testsupport.TestContext, "root", "expire", root, rootExpiration).Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "root", "set-threshold", root, "root", "1").Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "set-threshold", root, "snapshot", "1").Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "set-threshold", root, "targets", "1").Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "set-threshold", root, "timestamp", "1").Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "root", "gen-rsa-key", root, keyDir+"root.pem", "--role", "root").Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "gen-rsa-key", root, keyDir+"snapshot.pem", "--role", "snapshot").Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "gen-rsa-key", root, keyDir+"targets.pem", "--role", "targets").Run()).To(Succeed())
	Expect(tuftool.Command(testsupport.TestContext, "root", "gen-rsa-key", root, keyDir+"timestamp.pem", "--role", "timestamp").Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "root", "sign", root, "-k", keyDir+"root.pem").Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "create",
		"--root", root,
		"--key", keyDir+"root.pem",
		"--key", keyDir+"snapshot.pem",
		"--key", keyDir+"targets.pem",
		"--key", keyDir+"timestamp.pem",
		"--add-targets", input,
		"--targets-expires", targetsExpiration,
		"--targets-version", "1",
		"--snapshot-expires", snapshotExpiration,
		"--snapshot-version", "1",
		"--timestamp-expires", timestampExpiration,
		"--timestamp-version", "1",
		"--outdir", tufRepo).Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "rhtas",
		"--root", root,
		"--key", keyDir+"root.pem",
		"--key", keyDir+"snapshot.pem",
		"--key", keyDir+"targets.pem",
		"--key", keyDir+"timestamp.pem",
		"--set-ctlog-target", ctlogPubkeyPath,
		"--ctlog-uri", "https://ctlog.rhtas",
		"--targets-expires", targetsExpiration,
		"--targets-version", "1",
		"--snapshot-expires", snapshotExpiration,
		"--snapshot-version", "1",
		"--timestamp-expires", timestampExpiration,
		"--timestamp-version", "1",
		"--force-version",
		"--outdir", tufRepo,
		"--metadata-url", "file://"+tufRepo).Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "rhtas",
		"--root", root,
		"--key", keyDir+"root.pem",
		"--key", keyDir+"snapshot.pem",
		"--key", keyDir+"targets.pem",
		"--key", keyDir+"timestamp.pem",
		"--set-fulcio-target", fulcioCertPath,
		"--fulcio-uri", "https://fulcio.rhtas",
		"--targets-expires", targetsExpiration,
		"--targets-version", "1",
		"--snapshot-expires", snapshotExpiration,
		"--snapshot-version", "1",
		"--timestamp-expires", timestampExpiration,
		"--timestamp-version", "1",
		"--force-version",
		"--outdir", tufRepo,
		"--metadata-url", "file://"+tufRepo).Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "rhtas",
		"--root", root,
		"--key", keyDir+"root.pem",
		"--key", keyDir+"snapshot.pem",
		"--key", keyDir+"targets.pem",
		"--key", keyDir+"timestamp.pem",
		"--set-rekor-target", rekorPubkeyPath,
		"--rekor-uri", "https://rekor.rhtas",
		"--targets-expires", targetsExpiration,
		"--targets-version", "1",
		"--snapshot-expires", snapshotExpiration,
		"--snapshot-version", "1",
		"--timestamp-expires", timestampExpiration,
		"--timestamp-version", "1",
		"--force-version",
		"--outdir", tufRepo,
		"--metadata-url", "file://"+tufRepo).Run()).To(Succeed())

	Expect(tuftool.Command(testsupport.TestContext, "rhtas",
		"--root", root,
		"--key", keyDir+"root.pem",
		"--key", keyDir+"snapshot.pem",
		"--key", keyDir+"targets.pem",
		"--key", keyDir+"timestamp.pem",
		"--set-tsa-target", tsaCertPath,
		"--tsa-uri", "https://tsa.rhtas",
		"--targets-expires", targetsExpiration,
		"--targets-version", "1",
		"--snapshot-expires", snapshotExpiration,
		"--snapshot-version", "1",
		"--timestamp-expires", timestampExpiration,
		"--timestamp-version", "1",
		"--force-version",
		"--outdir", tufRepo,
		"--metadata-url", "file://"+tufRepo).Run()).To(Succeed())
}

func setupSecuresignDeployment(oc *clients.Oc) {

	if oc.Command(testsupport.TestContext, "get", "namespace", "trusted-artifact-signer").Run() == nil {
		Expect(oc.Command(testsupport.TestContext, "delete", "namespace", "trusted-artifact-signer").Run()).To(Succeed())
	} else {
		logrus.Info("Namespace trusted-artifact-signer does not exist, skipping deletion.")
	}
	Expect(oc.Command(testsupport.TestContext, "new-project", "trusted-artifact-signer").Run()).To(Succeed())

	Expect(oc.Command(testsupport.TestContext, "create", "-f", pvcPath).Run()).To(Succeed())

	podOverrides := (`{
		"apiVersion": "v1",
		"spec": {
		  "securityContext": {
			"runAsNonRoot": true,
			"seccompProfile": {
			  "type": "RuntimeDefault"
			}
		  },
		  "containers": [{
			"name": "dummy",
			"image": "registry.access.redhat.com/ubi9/httpd-24",
			"volumeMounts": [{
			  "mountPath": "/var/www/html",
			  "name": "tuf-mount"
			}],
			"securityContext": {
			  "allowPrivilegeEscalation": false,
			  "capabilities": {
				"drop": ["ALL"]
			  }
			}
		  }],
		  "volumes": [{
			"name": "tuf-mount",
			"persistentVolumeClaim": {
			  "claimName": "tuf"
			}
		  }]
		}
	  }`)

	var dummyPod = "dummy"
	Expect(oc.Command(testsupport.TestContext, "run", dummyPod, "--overrides", podOverrides, "--image=registry.access.redhat.com/ubi9/httpd-24").Run()).To(Succeed())

	Expect(oc.Command(testsupport.TestContext, "wait", "pod", dummyPod, "--for=condition=Ready").Run()).To(Succeed())

	Expect(oc.Command(testsupport.TestContext, "rsync", "--no-perms", tufRepo+"/", dummyPod+":/var/www/html").Run()).To(Succeed())

	Expect(oc.Command(testsupport.TestContext, "delete", "pod", "dummy").Run()).To(Succeed())

	Expect(oc.Command(testsupport.TestContext, "create", "secret", "generic", "ctfe-secret",
		"--from-file=private=targets/ctfe_private_key.pem",
		"--from-file=password=targets/ctfe-password.pem",
		"--from-file=public=targets/ctfe.pub").Run()).To(Succeed())

	Expect(oc.Command(testsupport.TestContext, "create", "secret", "generic", "fulcio-secret",
		"--from-file=cert=targets/fulcio_v1.crt.pem",
		"--from-file=password=targets/fulcio-password",
		"--from-file=private=targets/fulcio_private_key.pem").Run()).To(Succeed())

	Expect(oc.Command(testsupport.TestContext, "create", "secret", "generic", "rekor-secret",
		"--from-file=private=targets/rekor_private_key.pem").Run()).To(Succeed())

	Expect(oc.Command(testsupport.TestContext, "create", "secret", "generic", "tsa-secret",
		"--from-file=certificateChain=targets/tsa.certchain.pem",
		"--from-file=leafPrivateKeyPassword=targets/tsa-leaf-password",
		"--from-file=leafPrivateKey=targets/tsa_leaf_private_key.pem").Run()).To(Succeed())

	yamlData, err := os.ReadFile(securesignPath)
	Expect(err).ToNot(HaveOccurred())

	securesignContent := string(yamlData)
	securesignContent = strings.ReplaceAll(securesignContent, "OIDC_ISSUER_URL", api.GetValueFor(api.OidcIssuerURL))

	securesignTempPath := filepath.Join(workdir, "securesign.yaml")
	Expect(os.WriteFile(securesignTempPath, []byte(securesignContent), 0600)).To(Succeed())
	Expect(oc.Command(testsupport.TestContext, "create", "-f", securesignTempPath).Run()).To(Succeed())
}

var _ = AfterSuite(func() {
	Expect(os.RemoveAll(workdir)).To(Succeed())
})

func GetMandatoryAPIConfigValues(maxAttempts int) {
	interval := 20 * time.Second
	attempt := 1

	logrus.Info("Starting to check mandatory API config values")

	for attempt <= maxAttempts {
		logrus.Infof("Attempt %d/%d", attempt, maxAttempts)

		cmd := exec.Command("bash", "-c", envVariablesPath)
		cmd.Env = os.Environ()
		output, err := cmd.CombinedOutput()
		Expect(err).ToNot(HaveOccurred())

		outputStr := string(output)
		lines := strings.Split(outputStr, "\n")

		envVars := map[string]string{}
		for _, line := range lines {
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					envVars[parts[0]] = parts[1]
				}
			}
		}

		// Required environment variables
		requiredVars := []string{
			"TUF_URL", "OIDC_ISSUER_URL", "COSIGN_FULCIO_URL", "COSIGN_REKOR_URL",
			"COSIGN_MIRROR", "COSIGN_ROOT", "COSIGN_OIDC_CLIENT_ID", "COSIGN_OIDC_ISSUER",
			"COSIGN_CERTIFICATE_OIDC_ISSUER", "COSIGN_YES", "SIGSTORE_FULCIO_URL",
			"SIGSTORE_OIDC_ISSUER", "SIGSTORE_REKOR_URL", "REKOR_REKOR_SERVER",
			"SIGSTORE_OIDC_CLIENT_ID", "TSA_URL",
		}

		missingVars := []string{}
		for _, key := range requiredVars {
			if envVars[key] == "" {
				missingVars = append(missingVars, key)
			}
		}

		if len(missingVars) == 0 {
			logrus.Info("All mandatory API config values are set")
			return
		}

		logrus.Warnf("Missing variables: %v. Retrying in %v seconds", missingVars, interval.Seconds())

		if attempt == maxAttempts && len(missingVars) > 0 {
			Expect(missingVars).To(BeEmpty())
			break
		}

		time.Sleep(interval)
		attempt++
	}
	logrus.Errorf("mandatory API config values not set within %v seconds", maxAttempts*int(interval.Seconds()))

}
