package cosign

import (
	"context"
	"os/exec"
	"sigstore-e2e-test/pkg/client"
	"sigstore-e2e-test/pkg/support"
)

const COSIGN_REPO = "https://github.com/sigstore/cosign"

type cosignInstaller struct {
	ctx context.Context
}

func NewCosign(ctx context.Context) *cosignInstaller {
	return &cosignInstaller{
		ctx: ctx,
	}
}

func (p cosignInstaller) IsReady(_ client.Client) bool {
	path, _ := exec.LookPath("cosign")
	return path != ""
}

func (p cosignInstaller) Install(c client.Client) error {
	if p.IsReady(c) {
		return nil
	}

	// TODO: use PROD cli
	dir, _, err := support.GitClone(COSIGN_REPO)
	if err != nil {
		return err
	}
	return exec.CommandContext(p.ctx, "go", "install", dir+"/cmd/cosign").Run()
}

func (p cosignInstaller) Destroy(c client.Client) error {
	//no-op
	return nil
}
