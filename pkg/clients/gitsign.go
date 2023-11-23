package clients

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"os/exec"
	"path/filepath"
)

type Gitsign struct {
	*cli
}

func NewGitsign(ctx context.Context) *Gitsign {
	return &Gitsign{
		&cli{
			Name:  "gitsign",
			ctx:   ctx,
			setup: DownloadFromOpenshift("gitsign"),
		}}
}

func (c *Gitsign) GitWithGitSign(workdir string, signToken string, args ...string) error {
	cmd := exec.CommandContext(c.ctx, "git", args...)
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
