package cliserver

import (
	"context"
	"fmt"
	"runtime"

	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/sirupsen/logrus"
)

func init() {
	strategy.Register("cli_server", func() strategy.Strategy {
		server := api.GetValueFor(api.CLIServerURL)
		if server == "" {
			panic("CLI server URL not specified")
		}
		return func(ctx context.Context, cliName string) (string, error) {
			return download(ctx, server, cliName)
		}
	})
}

func download(ctx context.Context, server string, cliName string) (string, error) {
	logrus.Info("Getting binary '", cliName, "' from CLI server ", server)
	link := fmt.Sprintf("%s/clients/%s/%s-%s.gz", server, runtime.GOOS, cliName, runtime.GOARCH)
	return strategy.DownloadFromLink(ctx, cliName, link)
}
