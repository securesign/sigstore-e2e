package openshift

import (
	"context"
	"runtime"

	"github.com/securesign/sigstore-e2e/pkg/kubernetes"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/sirupsen/logrus"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	strategy.Register("openshift", func() strategy.Strategy {
		return func(ctx context.Context, cliName string) (string, error) {
			return download(ctx, kubernetes.GetClient(), cliName)
		}
	})
}

func download(ctx context.Context, client controller.Reader, cliName string) (string, error) {
	logrus.Info("Getting binary '", cliName, "' from Openshift")
	link, err := kubernetes.ConsoleCLIDownload(ctx, client, cliName, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	return strategy.DownloadFromLink(ctx, cliName, link)
}
