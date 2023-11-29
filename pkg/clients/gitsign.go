package clients

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

type Gitsign struct {
	*cli
}

func NewGitsign() *Gitsign {
	return &Gitsign{
		&cli{
			Name:  "gitsign",
			setup: DownloadFromOpenshift("gitsign"),
		}}
}

func (c *Gitsign) GitWithGitSign(ctx context.Context, workdir string, signToken string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return err
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("SIGSTORE_ID_TOKEN=%s", signToken), "PATH="+filepath.Dir(c.pathToCLI)+":"+filepath.Dir(gitPath))
	cmd.Dir = workdir
	cmd.Stdout = logrus.NewEntry(logrus.StandardLogger()).WithField("app", "git").WriterLevel(logrus.InfoLevel)
	cmd.Stderr = logrus.NewEntry(logrus.StandardLogger()).WithField("app", "git").WriterLevel(logrus.ErrorLevel)

	return cmd.Run()
}
