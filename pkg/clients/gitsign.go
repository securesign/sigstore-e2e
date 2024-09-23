package clients

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"
)

type Gitsign struct {
	*cli
}

func NewGitsign() *Gitsign {
	return &Gitsign{
		&cli{
			Name:           "gitsign",
			setupStrategy:  PreferredSetupStrategy(),
			versionCommand: "--version",
		}}
}

func (c *Gitsign) GitWithGitSign(ctx context.Context, workdir string, signToken string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return err
	}

	var pathSeparator string
	switch runtime.GOOS {
	case "windows": // semicolon for Windows
		pathSeparator = ";"
	case "linux", "darwin": // colon for Linux and macOS
		pathSeparator = ":"
	default:
		logrus.Fatal("Unsupported OS: " + runtime.GOOS)
	}

	cmd.Env = append(cmd.Env, "SIGSTORE_ID_TOKEN="+signToken, "PATH="+filepath.Dir(c.pathToCLI)+pathSeparator+filepath.Dir(gitPath))
	cmd.Dir = workdir
	cmd.Stdout = logrus.NewEntry(logrus.StandardLogger()).WithField("app", "git").WriterLevel(logrus.InfoLevel)
	cmd.Stderr = logrus.NewEntry(logrus.StandardLogger()).WithField("app", "git").WriterLevel(logrus.ErrorLevel)

	return cmd.Run()
}
