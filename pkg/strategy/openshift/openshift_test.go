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

func TestStrategy(t *testing.T) {
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

func TestStrategyError(t *testing.T) {
	fakeClient := newFakeClient(t).Build()

	_, err := download(t.Context(), fakeClient, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent CLI download")
	}
}
