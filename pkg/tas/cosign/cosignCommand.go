package cosign

import (
	"context"
	"os"
	"os/exec"
)

func Cosign(ctx context.Context, args ...string) error {
	path, err := exec.LookPath("cosign")
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stderr = os.Stdout
	cmd.Stdout = os.Stdout

	return cmd.Run()
}
