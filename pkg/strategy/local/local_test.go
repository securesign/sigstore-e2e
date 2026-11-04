package local

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
)

func TestRegistered(t *testing.T) {
	if !strategy.Has("local") {
		t.Fatal("local strategy not registered")
	}
}

// TestLocalDoesNotRequireCGWVersion pins that CLI_STRATEGY=local keeps working
// without CLI_CGW_VERSION set, as required by the external securesign/releases
// workflow that runs this suite with CLI_STRATEGY=local and never sets it.
func TestLocalDoesNotRequireCGWVersion(t *testing.T) {
	t.Setenv(api.CLICGWVersion, "")
	t.Setenv(api.CLILocalDir, "")

	s, ok := strategy.Get("local")
	if !ok {
		t.Fatal("local strategy not registered")
	}

	path, err := s(t.Context(), "go")
	if err != nil {
		t.Fatalf("expected to find 'go' on PATH: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestStrategy(t *testing.T) {
	t.Setenv(api.CLILocalDir, "")

	path, err := download(t.Context(), "go")
	if err != nil {
		t.Fatalf("expected to find 'go' on PATH: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	t.Logf("found go at %s", path)
}

func TestStrategyError(t *testing.T) {
	t.Setenv(api.CLILocalDir, "")

	_, err := download(t.Context(), "this-binary-does-not-exist-e2e-test")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

// TestLocalDirNoPathFallback proves that when CLI_LOCAL_DIR is set, a binary
// that genuinely exists on $PATH (like "go") is NOT resolved via $PATH if it
// is absent from CLI_LOCAL_DIR. This is the exact silent-fallback bug this
// change closes: resolution must be strictly confined to the given directory.
func TestLocalDirNoPathFallback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(api.CLILocalDir, dir)

	_, err := download(t.Context(), "go")
	if err == nil {
		t.Fatal("expected error: 'go' is not in CLI_LOCAL_DIR, must not fall back to $PATH")
	}
	if !strings.Contains(err.Error(), dir) {
		t.Fatalf("expected error to mention CLI_LOCAL_DIR %q, got: %v", dir, err)
	}
}

func TestLocalDirResolvesBinaryInDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(api.CLILocalDir, dir)

	fileName := "mycli"
	if runtime.GOOS == "windows" {
		fileName += ".exe"
	}

	// resolveInDir rejects a file without an executable bit, so the fixture has
	// to carry one for this test to reach the success path at all. It lives in
	// a per-test temp dir that the framework removes afterwards.
	binPath := filepath.Join(dir, fileName)
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil { //nolint:gosec // G306: an executable fixture is the point of the test
		t.Fatalf("failed to write fake binary: %v", err)
	}

	wantPath, err := filepath.Abs(binPath)
	if err != nil {
		t.Fatalf("failed to compute absolute path: %v", err)
	}

	gotPath, err := download(t.Context(), "mycli")
	if err != nil {
		t.Fatalf("expected to resolve 'mycli' from CLI_LOCAL_DIR: %v", err)
	}
	if gotPath != wantPath {
		t.Fatalf("expected path %q, got %q", wantPath, gotPath)
	}
}

func TestLocalDirUnsetUsesPath(t *testing.T) {
	t.Setenv(api.CLILocalDir, "")

	path, err := download(t.Context(), "go")
	if err != nil {
		t.Fatalf("expected to find 'go' on PATH: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}
