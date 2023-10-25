package gitsign

import (
	"context"
	"os/exec"
	"sigstore-e2e-test/pkg/client"
)

var gitsignPath string

type Gitsign struct {
	ctx context.Context
}

func NewGitsignInstaller(ctx context.Context) *Gitsign {
	return &Gitsign{
		ctx: ctx,
	}
}

func (p Gitsign) Install(c client.Client) error {
	path, err := exec.LookPath("gitsign")
	if err != nil {
		return err
	}
	if path != "" {
		// already installed
		gitsignPath = path
		return nil
	}

	return exec.CommandContext(p.ctx, "go", "install", "github.com/sigstore/gitsign@latest").Run()
}

func (p Gitsign) Destroy(c client.Client) error {
	//no-op
	return nil
}
