package local

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/sirupsen/logrus"
)

func init() {
	strategy.Register("local", func() strategy.Strategy {
		return download
	})
}

func download(_ context.Context, cliName string) (string, error) {
	if dir := api.GetValueFor(api.CLILocalDir); dir != "" {
		return resolveInDir(dir, cliName)
	}

	logrus.Info("Checking local binary '", cliName, "'")
	return exec.LookPath(cliName)
}

// resolveInDir resolves cliName strictly within dir, with no $PATH fallback.
// A missing, non-regular, or non-executable binary is a hard error rather than
// a silent fallback to an unrelated copy found elsewhere on $PATH.
func resolveInDir(dir, cliName string) (string, error) {
	fileName := cliName
	if runtime.GOOS == "windows" {
		fileName += ".exe"
	}

	path := filepath.Join(dir, fileName)

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("CLI_LOCAL_DIR %q: expected binary %q not found: %w", dir, fileName, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("CLI_LOCAL_DIR %q: expected binary %q is a directory", dir, fileName)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("CLI_LOCAL_DIR %q: binary %q is not executable", dir, fileName)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("CLI_LOCAL_DIR %q: resolving absolute path for %q: %w", dir, fileName, err)
	}

	logrus.Info("Using local binary '", cliName, "' from CLI_LOCAL_DIR: ", absPath)
	return absPath, nil
}
