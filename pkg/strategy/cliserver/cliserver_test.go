package cliserver

import (
	"runtime"
	"testing"

	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/securesign/sigstore-e2e/pkg/strategy/testutil"
)

func TestRegistered(t *testing.T) {
	if !strategy.Has("cli_server") {
		t.Fatal("cli_server strategy not registered")
	}
}

func TestStrategy(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho hello\n")
	gzipped := testutil.GzipBytes(t, binaryContent)
	expectedPath := "/clients/" + runtime.GOOS + "/testcli-" + runtime.GOARCH + ".gz"

	srv := testutil.ServeBinary(t, expectedPath, gzipped)

	path, err := download(t.Context(), srv.URL, "testcli")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	testutil.VerifyBinary(t, path, binaryContent)
	t.Logf("OK: testcli -> %s", path)
}

func TestStrategyError(t *testing.T) {
	gzipped := []byte("not valid gzip data")
	srv := testutil.ServeBinary(t, "/clients/"+runtime.GOOS+"/nonexistent-"+runtime.GOARCH+".gz", gzipped)

	_, err := download(t.Context(), srv.URL, "nonexistent")
	if err == nil {
		t.Fatal("expected error for invalid gzip response")
	}
}
