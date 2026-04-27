package local

import (
	"testing"

	"github.com/securesign/sigstore-e2e/pkg/strategy"
)

func TestRegistered(t *testing.T) {
	if !strategy.Has("local") {
		t.Fatal("local strategy not registered")
	}
}

func TestStrategy(t *testing.T) {
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
	_, err := download(t.Context(), "this-binary-does-not-exist-e2e-test")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}
