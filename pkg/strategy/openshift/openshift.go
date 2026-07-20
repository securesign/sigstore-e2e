package openshift

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
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

const (
	prodHost        = "developers.redhat.com"
	stagingHost     = "developers.qa.redhat.com"
	fallbackVersion = "1.4.1"
)

var versionRegexp = regexp.MustCompile(`/RHTAS/[^/]+/`)

func download(ctx context.Context, client controller.Reader, cliName string) (string, error) {
	logrus.Info("Getting binary '", cliName, "' from Openshift")
	link, err := kubernetes.ConsoleCLIDownload(ctx, client, cliName, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	if isTarGz(link) {
		path, err := downloadTarGz(ctx, cliName, link)
		if err != nil && strings.Contains(link, prodHost) {
			stagingLink := strings.Replace(link, prodHost, stagingHost, 1)
			logrus.Infof("Production download failed, falling back to staging: %s", stagingLink)
			path, err = downloadTarGz(ctx, cliName, stagingLink)
			if err != nil {
				fallbackLink := versionRegexp.ReplaceAllString(link, "/RHTAS/"+fallbackVersion+"/")
				logrus.Infof("Staging download failed, falling back to stable %s via CDN: %s", fallbackVersion, fallbackLink)
				cdnLink, cdnErr := support.ResolveCDNLink(ctx, fallbackLink)
				if cdnErr != nil {
					return "", fmt.Errorf("all download attempts failed (prod, staging, CDN fallback): %w", cdnErr)
				}
				logrus.Infof("Resolved CDN link: %s", cdnLink)
				return downloadTarGz(ctx, cliName, cdnLink)
			}
			return path, nil
		}
		return path, err
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
