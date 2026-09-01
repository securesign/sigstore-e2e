package clients

import (
	"github.com/securesign/sigstore-e2e/pkg/strategy/goinstall"
)

type Tufcli struct {
	*cli
}

// NewTufcli uses go install until tufcli is available as a binary in TAS 1.5.
func NewTufcli() *Tufcli {
	return &Tufcli{
		&cli{
			Name:           "tufcli",
			setupStrategy:  goinstall.ForModule("github.com/securesign/tufcli", "latest"),
			versionCommand: "--version",
		}}
}
