package openshift

import (
	"context"
	"net/url"
	"os"
	"runtime"
	"strings"

	"github.com/securesign/sigstore-e2e/pkg/kubernetes"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/securesign/sigstore-e2e/pkg/support"
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

	if isTarGz(link) {
		return downloadTarGz(ctx, cliName, link)
	}
	return strategy.DownloadFromLink(ctx, cliName, link)
}

func isTarGz(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return strings.HasSuffix(link, ".tar.gz")
	}
	return strings.HasSuffix(u.Path, ".tar.gz")
}

func downloadTarGz(ctx context.Context, cliName string, link string) (string, error) {
	logrus.Info("Downloading ", cliName, " from ", link)

	tmp, err := os.MkdirTemp("", cliName)
	if err != nil {
		return "", err
	}

	if err = support.DownloadAndUntarArchive(ctx, link, tmp); err != nil {
		return "", err
	}

	return support.FindBinary(tmp, cliName, runtime.GOOS, runtime.GOARCH)
}
