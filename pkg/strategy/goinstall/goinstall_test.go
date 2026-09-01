package goinstall

import (
	"os/exec"
	"testing"

	"github.com/securesign/sigstore-e2e/pkg/strategy"
)

func TestRegistered(t *testing.T) {
	if !strategy.Has("goinstall") {
		t.Fatal("goinstall strategy not registered")
	}
}

func TestInstall(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}

	path, err := install(t.Context(), "github.com/securesign/tufcli", "v0.0.1-rc1", "tufcli")
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	t.Logf("OK: tufcli -> %s", path)
}

func TestInstallError(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}

	_, err := install(t.Context(), "github.com/nonexistent/pkg-e2e-test-xyz", "latest", "nope")
	if err == nil {
		t.Fatal("expected error for nonexistent module")
	}
}
