package gitsign

import (
	"context"
	"os/exec"
	"sigstore-e2e-test/pkg/client"
)

var gitsignPath string

type cosignInstaller struct {
	ctx context.Context
}

func NewGitsignInstaller(ctx context.Context) *cosignInstaller {
	return &cosignInstaller{
		ctx: ctx,
	}
}

func (p cosignInstaller) Install(c client.Client) error {
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

func (p cosignInstaller) Destroy(c client.Client) error {
	//no-op
	return nil
}
