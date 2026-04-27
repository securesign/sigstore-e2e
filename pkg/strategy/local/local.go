package local

import (
	"context"
	"os/exec"

	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/sirupsen/logrus"
)

func init() {
	strategy.Register("local", func() strategy.Strategy {
		return download
	})
}

func download(_ context.Context, cliName string) (string, error) {
	logrus.Info("Checking local binary '", cliName, "'")
	return exec.LookPath(cliName)
}
