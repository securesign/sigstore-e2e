package cgw

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/securesign/sigstore-e2e/pkg/strategy/testutil"
	"github.com/securesign/sigstore-e2e/pkg/support"
)

func TestContentGatewayName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"cosign", "cosign"},
		{"gitsign", "gitsign_cli"},
		{"rekor-cli", "rekor_cli"},
		{"ec", "ec"},
		{"tuftool", "tuftool"},
		{"createtree", "createtree"},
		{"some-other-tool", "some_other_tool"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := support.ContentGatewayName(tt.input)
			if got != tt.expected {
				t.Errorf("ContentGatewayName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRegistered(t *testing.T) {
	if !strategy.Has("cgw") {
		t.Fatal("cgw strategy not registered")
	}
}

func TestStrategy(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho cosign\n")
	archiveName := fmt.Sprintf("%s_%s_%s.tar.gz", support.ContentGatewayName("cosign"), runtime.GOOS, runtime.GOARCH)

	archive := testutil.BuildTarGz(t, map[string][]byte{
		"cosign": binaryContent,
	})

	srv := testutil.ServeBinary(t, "/"+archiveName, archive)

	path, err := download(t.Context(), srv.URL, "cosign")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	testutil.VerifyBinary(t, path, binaryContent)
	t.Logf("OK: cosign -> %s", path)
}

func TestStrategyNameOverride(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho gitsign\n")
	cgwName := support.ContentGatewayName("gitsign")
	archiveName := fmt.Sprintf("%s_%s_%s.tar.gz", cgwName, runtime.GOOS, runtime.GOARCH)
	binaryName := fmt.Sprintf("%s_%s_%s", cgwName, runtime.GOOS, runtime.GOARCH)

	archive := testutil.BuildTarGz(t, map[string][]byte{
		binaryName: binaryContent,
	})

	srv := testutil.ServeBinary(t, "/"+archiveName, archive)

	path, err := download(t.Context(), srv.URL, "gitsign")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	testutil.VerifyBinary(t, path, binaryContent)
}

func TestStrategyError(t *testing.T) {
	archive := testutil.BuildTarGz(t, map[string][]byte{
		"wrong-name": []byte("data"),
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	}))
	t.Cleanup(srv.Close)

	_, err := download(t.Context(), srv.URL, "cosign")
	if err == nil {
		t.Fatal("expected error when binary not found in archive")
	}
}
