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

func (p Gitsign) IsReady(_ client.Client) bool {
	path, _ := exec.LookPath("gitsign")
	return path != ""
}

func (p Gitsign) Install(_ client.Client) error {
	// TODO: use PROD cli
	return exec.CommandContext(p.ctx, "go", "install", "github.com/sigstore/gitsign@latest").Run()
}

func (p Gitsign) Destroy(c client.Client) error {
	//no-op
	return nil
}
