package openshift

import (
	"runtime"
	"testing"

	consoleV1 "github.com/openshift/api/console/v1"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/securesign/sigstore-e2e/pkg/strategy/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newFakeClient(t *testing.T, objects ...consoleV1.ConsoleCLIDownload) *fake.ClientBuilder {
	t.Helper()
	scheme := k8sruntime.NewScheme()
	if err := consoleV1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	builder := fake.NewClientBuilder().WithScheme(scheme)
	for i := range objects {
		builder = builder.WithObjects(&objects[i])
	}
	return builder
}

func TestRegistered(t *testing.T) {
	if !strategy.Has("openshift") {
		t.Fatal("openshift strategy not registered")
	}
}

func TestStrategyCliServer(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho testcli\n")
	gzipped := testutil.GzipBytes(t, binaryContent)
	expectedPath := "/clients/" + runtime.GOOS + "/testcli-" + runtime.GOARCH + ".gz"

	srv := testutil.ServeBinary(t, expectedPath, gzipped)

	cliDownload := consoleV1.ConsoleCLIDownload{
		ObjectMeta: metav1.ObjectMeta{Name: "testcli"},
		Spec: consoleV1.ConsoleCLIDownloadSpec{
			DisplayName: "Test CLI",
			Description: "test binary",
			Links: []consoleV1.CLIDownloadLink{
				{Text: "Linux AMD64", Href: srv.URL + "/clients/linux/testcli-amd64.gz"},
				{Text: "Linux ARM64", Href: srv.URL + "/clients/linux/testcli-arm64.gz"},
				{Text: "macOS AMD64", Href: srv.URL + "/clients/darwin/testcli-amd64.gz"},
				{Text: "macOS ARM64", Href: srv.URL + "/clients/darwin/testcli-arm64.gz"},
				{Text: "Windows AMD64", Href: srv.URL + "/clients/windows/testcli-amd64.gz"},
			},
		},
	}

	fakeClient := newFakeClient(t, cliDownload).Build()

	path, err := download(t.Context(), fakeClient, "testcli")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	testutil.VerifyBinary(t, path, binaryContent)
	t.Logf("OK: testcli -> %s", path)
}

func TestStrategyContentGateway(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho testcli\n")
	binaryName := "testcli_" + runtime.GOOS + "_" + runtime.GOARCH
	tarGz := testutil.BuildTarGz(t, map[string][]byte{binaryName: binaryContent})
	expectedPath := "/cgw/RHTAS/1.4.0/testcli_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"

	srv := testutil.ServeBinary(t, expectedPath, tarGz)

	cliDownload := consoleV1.ConsoleCLIDownload{
		ObjectMeta: metav1.ObjectMeta{Name: "testcli"},
		Spec: consoleV1.ConsoleCLIDownloadSpec{
			DisplayName: "testcli - Command Line Interface (CLI)",
			Description: "test binary",
			Links: []consoleV1.CLIDownloadLink{
				{Text: "Download testcli for Linux x86_64", Href: srv.URL + "/cgw/RHTAS/1.4.0/testcli_linux_amd64.tar.gz"},
				{Text: "Download testcli for Linux arm64", Href: srv.URL + "/cgw/RHTAS/1.4.0/testcli_linux_arm64.tar.gz"},
				{Text: "Download testcli for Mac x86_64", Href: srv.URL + "/cgw/RHTAS/1.4.0/testcli_darwin_amd64.tar.gz"},
				{Text: "Download testcli for Mac arm64", Href: srv.URL + "/cgw/RHTAS/1.4.0/testcli_darwin_arm64.tar.gz"},
				{Text: "Download testcli for Windows x86_64", Href: srv.URL + "/cgw/RHTAS/1.4.0/testcli_windows_amd64.zip"},
			},
		},
	}

	fakeClient := newFakeClient(t, cliDownload).Build()

	path, err := download(t.Context(), fakeClient, "testcli")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	testutil.VerifyBinary(t, path, binaryContent)
	t.Logf("OK: testcli -> %s", path)
}

func TestStrategyContentGatewayNameOverride(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho gitsign\n")
	binaryName := "gitsign_cli_" + runtime.GOOS + "_" + runtime.GOARCH
	tarGz := testutil.BuildTarGz(t, map[string][]byte{binaryName: binaryContent})
	expectedPath := "/cgw/RHTAS/1.4.0/gitsign_cli_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"

	srv := testutil.ServeBinary(t, expectedPath, tarGz)

	cliDownload := consoleV1.ConsoleCLIDownload{
		ObjectMeta: metav1.ObjectMeta{Name: "gitsign"},
		Spec: consoleV1.ConsoleCLIDownloadSpec{
			DisplayName: "gitsign - Command Line Interface (CLI)",
			Description: "gitsign is a CLI tool that allows you to digitally sign and verify git commits.",
			Links: []consoleV1.CLIDownloadLink{
				{Text: "Download gitsign for Linux x86_64", Href: srv.URL + "/cgw/RHTAS/1.4.0/gitsign_cli_linux_amd64.tar.gz"},
				{Text: "Download gitsign for Linux arm64", Href: srv.URL + "/cgw/RHTAS/1.4.0/gitsign_cli_linux_arm64.tar.gz"},
				{Text: "Download gitsign for Mac x86_64", Href: srv.URL + "/cgw/RHTAS/1.4.0/gitsign_cli_darwin_amd64.tar.gz"},
				{Text: "Download gitsign for Mac arm64", Href: srv.URL + "/cgw/RHTAS/1.4.0/gitsign_cli_darwin_arm64.tar.gz"},
				{Text: "Download gitsign for Windows x86_64", Href: srv.URL + "/cgw/RHTAS/1.4.0/gitsign_cli_windows_amd64.zip"},
			},
		},
	}

	fakeClient := newFakeClient(t, cliDownload).Build()

	path, err := download(t.Context(), fakeClient, "gitsign")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	testutil.VerifyBinary(t, path, binaryContent)
	t.Logf("OK: gitsign -> %s", path)
}

func TestStrategyErrorNotFound(t *testing.T) {
	fakeClient := newFakeClient(t).Build()

	_, err := download(t.Context(), fakeClient, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent CLI download")
	}
}

func TestStrategyErrorNoMatchingLink(t *testing.T) {
	cliDownload := consoleV1.ConsoleCLIDownload{
		ObjectMeta: metav1.ObjectMeta{Name: "testcli"},
		Spec: consoleV1.ConsoleCLIDownloadSpec{
			DisplayName: "Test CLI",
			Description: "test binary",
			Links: []consoleV1.CLIDownloadLink{
				{Text: "Download for SomeOtherOS", Href: "https://example.com/testcli_fakeos_fakearch.tar.gz"},
			},
		},
	}

	fakeClient := newFakeClient(t, cliDownload).Build()

	_, err := download(t.Context(), fakeClient, "testcli")
	if err == nil {
		t.Fatal("expected error when no link matches the current OS/arch")
	}
	t.Logf("OK: got expected error: %v", err)
}
