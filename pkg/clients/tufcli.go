package clients

import (
	"context"

	"github.com/securesign/sigstore-e2e/pkg/strategy/goinstall"
	"github.com/sirupsen/logrus"
)

type Tufcli struct {
	*cli
}

// NewTufcli tries the preferred strategy (e.g. CGW) first; falls back to
// go install until tufcli is available as a binary in TAS 1.5.
func NewTufcli() *Tufcli {
	return &Tufcli{
		&cli{
			Name: "tufcli",
			setupStrategy: withFallback(
				PreferredSetupStrategy(),
				goinstall.ForModule("github.com/securesign/tufcli", "latest"),
			),
			versionCommand: "--version",
		}}
}

func withFallback(primary, fallback SetupStrategy) SetupStrategy {
	return func(ctx context.Context, cliName string) (string, error) {
		path, err := primary(ctx, cliName)
		if err == nil {
			return path, nil
		}
		logrus.Warnf("Primary strategy failed for %s: %v; falling back to go install", cliName, err)
		return fallback(ctx, cliName)
	}
}
