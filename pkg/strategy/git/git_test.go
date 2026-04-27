package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/securesign/sigstore-e2e/pkg/strategy"
)

func TestRegistered(t *testing.T) {
	if !strategy.Has("git") {
		t.Fatal("git strategy not registered")
	}
}

func TestStrategy(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repoDir := t.TempDir()

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...) //nolint:gosec
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("%v failed: %v", args, err)
		}
	}

	run(repoDir, "git", "init", "-b", "main")
	run(repoDir, "git", "config", "user.email", "test@test.com")
	run(repoDir, "git", "config", "user.name", "Test")

	goMod := "module example.com/testcli\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(goMod), 0600); err != nil {
		t.Fatal(err)
	}

	mainGo := "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\") }\n"
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(mainGo), 0600); err != nil {
		t.Fatal(err)
	}

	run(repoDir, "git", "add", ".")
	run(repoDir, "git", "commit", "-m", "init")

	path, err := cloneAndBuild(t.Context(), "file://"+repoDir, "main", ".", "testcli")
	if err != nil {
		t.Fatalf("cloneAndBuild failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("binary not found at %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("binary at %s is empty", path)
	}
	if info.Mode()&0111 == 0 {
		t.Fatalf("binary at %s is not executable (mode: %s)", path, info.Mode())
	}
	t.Logf("OK: testcli -> %s (%d bytes)", path, info.Size())
}

func TestStrategyError(t *testing.T) {
	_, err := cloneAndBuild(t.Context(), "file:///nonexistent-repo-path-e2e-test", "main", ".", "testcli")
	if err == nil {
		t.Fatal("expected error for nonexistent git repo")
	}
}
