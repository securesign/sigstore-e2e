package git

import (
	"context"
	"os/exec"

	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/securesign/sigstore-e2e/pkg/support"
	"github.com/sirupsen/logrus"
)

func init() {
	strategy.Register("git", func() strategy.Strategy {
		url := api.GetValueFor(api.GitURL)
		if url == "" {
			panic("Git URL (GIT_URL) not specified")
		}
		branch := api.GetValueFor(api.GitBranch)
		if branch == "" {
			panic("Git branch (GIT_BRANCH) not specified")
		}
		buildDir := api.GetValueFor(api.GitBuildDir)
		if buildDir == "" {
			panic("Git build directory (GIT_BUILD_DIR) not specified")
		}
		return func(ctx context.Context, cliName string) (string, error) {
			return cloneAndBuild(ctx, url, branch, buildDir, cliName)
		}
	})
}

func cloneAndBuild(ctx context.Context, url string, branch string, buildDir string, cliName string) (string, error) {
	logrus.Info("Building '", cliName, "' from git: ", url, ", branch ", branch)
	dir, _, err := support.GitClone(url, branch)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-C", dir, "-o", cliName, buildDir) //nolint:gosec
	cmd.Stdout = logrus.NewEntry(logrus.StandardLogger()).WithField("app", cliName).WriterLevel(logrus.InfoLevel)
	cmd.Stderr = logrus.NewEntry(logrus.StandardLogger()).WithField("app", cliName).WriterLevel(logrus.ErrorLevel)
	err = cmd.Run()

	return dir + "/" + cliName, err
}
