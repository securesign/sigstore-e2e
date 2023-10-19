package cosign

import (
	"context"
	"os/exec"
	"sigstore-e2e-test/pkg/client"
	"sigstore-e2e-test/pkg/tas"
)

var cosign string

const COSIGN_REPO = "https://github.com/sigstore/cosign"

type cosignInstaller struct {
	ctx context.Context
}

func NewCosign(ctx context.Context) *cosignInstaller {
	return &cosignInstaller{
		ctx: ctx,
	}
}

func (p cosignInstaller) Install(c client.Client) error {
	path, err := exec.LookPath("cosign")
	if err != nil {
		return err
	}
	if path != "" {
		// already installed
		cosign = path
		return nil
	}

	dir, err := tas.GitClone(COSIGN_REPO)
	if err != nil {
		return err
	}
	return exec.CommandContext(p.ctx, "go", "install", dir+"/cmd/cosign").Run()
}

func (p cosignInstaller) Destroy(c client.Client) error {
	//no-op
	return nil
}
