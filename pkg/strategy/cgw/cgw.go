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

// extractArchive downloads and extracts the archive at link into dst. Windows binaries
// are distributed as .zip on the content gateway, everything else as .tar.gz.
func extractArchive(ctx context.Context, link string, dst string) error {
	if runtime.GOOS == "windows" {
		return support.DownloadAndUnzipArchive(ctx, link, dst)
	}
	return support.DownloadAndUntarArchive(ctx, link, dst)
}

func download(ctx context.Context, cgwURL string, cliName string) (string, error) {
	cgwName := support.ContentGatewayName(cliName)
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	archiveName := fmt.Sprintf("%s_%s_%s.%s", cgwName, runtime.GOOS, runtime.GOARCH, ext)
	link := fmt.Sprintf("%s/%s", strings.TrimRight(cgwURL, "/"), archiveName)

	logrus.Info("Getting binary '", cliName, "' from content gateway: ", link)

	tmp, err := os.MkdirTemp("", cliName)
	if err != nil {
		return "", err
	}

	if err = extractArchive(ctx, link, tmp); err != nil {
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
		if err = extractArchive(ctx, cdnLink, tmp); err != nil {
			_ = os.RemoveAll(tmp)
			return "", err
		}
	}

	return support.FindBinary(tmp, cliName, runtime.GOOS, runtime.GOARCH)
}
