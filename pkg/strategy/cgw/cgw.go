package cgw

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/securesign/sigstore-e2e/pkg/support"
	"github.com/sirupsen/logrus"
)

func init() {
	strategy.Register("cgw", func() strategy.Strategy {
		cgwURL := api.GetValueFor(api.CGWURL)
		if cgwURL == "" {
			panic("Content gateway URL (CGW_URL) not specified")
		}
		return func(ctx context.Context, cliName string) (string, error) {
			return download(ctx, cgwURL, cliName)
		}
	})
}

func download(ctx context.Context, cgwURL string, cliName string) (string, error) {
	cgwName := support.ContentGatewayName(cliName)
	archiveName := fmt.Sprintf("%s_%s_%s.tar.gz", cgwName, runtime.GOOS, runtime.GOARCH)
	link := fmt.Sprintf("%s/%s", strings.TrimRight(cgwURL, "/"), archiveName)

	logrus.Info("Getting binary '", cliName, "' from content gateway: ", link)

	tmp, err := os.MkdirTemp("", cliName)
	if err != nil {
		return "", err
	}

	if err = support.DownloadAndUntarArchive(ctx, link, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		logrus.Infof("Direct download failed, resolving CDN link for: %s", link)
		cdnLink, cdnErr := support.ResolveCDNLink(ctx, link)
		if cdnErr != nil {
			return "", fmt.Errorf("download failed and CDN resolution failed: %w", cdnErr)
		}
		logrus.Infof("Resolved CDN link: %s", cdnLink)
		tmp, err = os.MkdirTemp("", cliName)
		if err != nil {
			return "", err
		}
		if err = support.DownloadAndUntarArchive(ctx, cdnLink, tmp); err != nil {
			_ = os.RemoveAll(tmp)
			return "", err
		}
	}

	return support.FindBinary(tmp, cliName, runtime.GOOS, runtime.GOARCH)
}
