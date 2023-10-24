package gitsign

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Gitsign(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, gitsignPath, args...)
	cmd.Stderr = os.Stdout
	cmd.Stdout = os.Stdout

	return cmd.Run()
}

func GitWithGitSign(ctx context.Context, workdir string, signToken string, args ...string) error {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "git "+strings.Join(args, " "))
	gitsignPath, err := exec.LookPath("gitsign")
	if err != nil {
		return err
	}
	gitPath, err := exec.LookPath("git")
	cmd.Env = append(cmd.Env, "SIGSTORE_ID_TOKEN="+signToken, "PATH=$PATH:"+filepath.Dir(gitsignPath)+":"+filepath.Dir(gitPath))
	cmd.Dir = workdir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	return cmd.Run()
}
