package goinstall

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/sirupsen/logrus"
)

func init() {
	strategy.Register("goinstall", func() strategy.Strategy {
		module := api.GetValueFor(api.GoInstallModule)
		if module == "" {
			panic("Go module (GOINSTALL_MODULE) not specified")
		}
		version := api.GetValueFor(api.GoInstallVersion)
		if version == "" {
			version = "latest"
		}
		return func(ctx context.Context, cliName string) (string, error) {
			return install(ctx, module, version, cliName)
		}
	})
}

func ForModule(module, version string) strategy.Strategy {
	return func(ctx context.Context, cliName string) (string, error) {
		return install(ctx, module, version, cliName)
	}
}

func install(ctx context.Context, module string, version string, cliName string) (string, error) {
	ref := module + "@" + version
	logrus.Info("Installing '", cliName, "' via go install: ", ref)

	cmd := exec.CommandContext(ctx, "go", "install", ref) //nolint:gosec
	cmd.Stdout = logrus.NewEntry(logrus.StandardLogger()).WithField("app", cliName).WriterLevel(logrus.InfoLevel)
	cmd.Stderr = logrus.NewEntry(logrus.StandardLogger()).WithField("app", cliName).WriterLevel(logrus.ErrorLevel)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	bin := cliName
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	path := filepath.Join(goBinDir(), bin)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("go install succeeded but binary not found at %s: %w", path, err)
	}
	return path, nil
}

// goBinDir returns the directory where go install places binaries:
// $GOBIN if set, otherwise $GOPATH/bin (defaulting to $HOME/go/bin).
func goBinDir() string {
	if dir := os.Getenv("GOBIN"); dir != "" {
		return dir
	}
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = filepath.Join(home, "go")
	}
	return filepath.Join(strings.Split(gopath, string(os.PathListSeparator))[0], "bin")
}
