package cosign

import (
	"context"
	"os"
	"os/exec"
)

func Cosign(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, cosign, args...)
	cmd.Stderr = os.Stdout
	cmd.Stdout = os.Stdout

	return cmd.Run()
}
