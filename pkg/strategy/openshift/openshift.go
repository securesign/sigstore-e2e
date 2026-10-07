package openshift

import (
	"context"
	"fmt"
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

// extractFunc downloads and extracts an archive at link, returning the path to
// the extracted binary named cliName.
type extractFunc func(ctx context.Context, cliName string, link string) (string, error)

func download(ctx context.Context, client controller.Reader, cliName string) (string, error) {
	logrus.Info("Getting binary '", cliName, "' from Openshift")
	link, err := kubernetes.ConsoleCLIDownload(ctx, client, cliName, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	switch {
	case isTarGz(link):
		return downloadArchive(ctx, cliName, link, downloadTarGz)
	case isZip(link):
		return downloadArchive(ctx, cliName, link, downloadZip)
	default:
		return strategy.DownloadFromLink(ctx, cliName, link)
	}
}

// downloadArchive extracts the archive at link using extract.
//
// A content gateway file URL never serves the archive itself: it redirects to an
// HTML interstitial page that carries the real CDN location in its tcDownloadURL
// query parameter. Dereferencing the file URL with a redirect-following client
// therefore yields HTML, which every extractor rejects. Resolve the CDN link
// first so that the release under test is what actually gets downloaded.
//
// Links that do not point at the content gateway (cli-server, test servers) are
// downloaded as-is.
func downloadArchive(ctx context.Context, cliName string, link string, extract extractFunc) (string, error) {
	if support.IsContentGatewayLink(link) {
		cdnLink, err := support.ResolveCDNLink(ctx, link)
		if err != nil {
			return "", fmt.Errorf("resolving content gateway link %s: %w", link, err)
		}
		logrus.Infof("Resolved CDN link: %s", cdnLink)
		link = cdnLink
	}
	return extract(ctx, cliName, link)
}

func isTarGz(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return strings.HasSuffix(link, ".tar.gz")
	}
	return strings.HasSuffix(u.Path, ".tar.gz")
}

func isZip(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return strings.HasSuffix(link, ".zip")
	}
	return strings.HasSuffix(u.Path, ".zip")
}

func downloadTarGz(ctx context.Context, cliName string, link string) (string, error) {
	logrus.Info("Downloading ", cliName, " from ", link)

	tmp, err := os.MkdirTemp("", cliName)
	if err != nil {
		return "", err
	}

	if err = support.DownloadAndUntarArchive(ctx, link, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}

	return support.FindBinary(tmp, cliName, runtime.GOOS, runtime.GOARCH)
}

func downloadZip(ctx context.Context, cliName string, link string) (string, error) {
	logrus.Info("Downloading ", cliName, " from ", link)

	tmp, err := os.MkdirTemp("", cliName)
	if err != nil {
		return "", err
	}

	if err = support.DownloadAndUnzipArchive(ctx, link, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}

	return support.FindBinary(tmp, cliName, runtime.GOOS, runtime.GOARCH)
}
