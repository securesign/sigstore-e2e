package clients

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Gitsign struct {
	*cli
}

func NewGitsign(ctx context.Context) *Gitsign {
	return &Gitsign{
		&cli{
			Name:      "gitsign",
			ctx:       ctx,
			gitUrl:    "https://github.com/securesign/gitsign",
			gitBranch: "redhat-v0.7.1",
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	return cmd.Run()
}
