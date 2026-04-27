package strategy

import (
	"context"
	"testing"
)

func TestRegisterAndGet(t *testing.T) {
	oldRegistry := registry
	registry = map[string]Factory{}
	defer func() { registry = oldRegistry }()

	Register("test_strategy", func() Strategy {
		return func(_ context.Context, cliName string) (string, error) {
			return "/tmp/" + cliName, nil
		}
	})

	s, ok := Get("test_strategy")
	if !ok {
		t.Fatal("expected strategy to be registered")
	}

	path, err := s(t.Context(), "mytool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/tmp/mytool" {
		t.Fatalf("expected /tmp/mytool, got %s", path)
	}
}

func TestGetUnknown(t *testing.T) {
	oldRegistry := registry
	registry = map[string]Factory{}
	defer func() { registry = oldRegistry }()

	_, ok := Get("nonexistent")
	if ok {
		t.Fatal("expected ok=false for unregistered strategy")
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	oldRegistry := registry
	registry = map[string]Factory{}
	defer func() { registry = oldRegistry }()

	factory := func() Strategy {
		return func(_ context.Context, _ string) (string, error) { return "", nil }
	}
	Register("dup", factory)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	Register("dup", factory)
}
