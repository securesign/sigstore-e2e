package gitsign

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func GitWithGitSign(ctx context.Context, workdir string, signToken string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return err
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("SIGSTORE_ID_TOKEN=%s", signToken), "PATH="+filepath.Dir(gitsignPath)+":"+filepath.Dir(gitPath))
	cmd.Dir = workdir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	return cmd.Run()
}
