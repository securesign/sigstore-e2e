package cgw

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/securesign/sigstore-e2e/pkg/support"
	"github.com/sirupsen/logrus"
)

var cgwNameOverride = map[string]string{
	"gitsign":   "gitsign_cli",
	"rekor-cli": "rekor_cli",
}

func contentGatewayName(name string) string {
	if override, ok := cgwNameOverride[name]; ok {
		return override
	}
	return strings.ReplaceAll(name, "-", "_")
}

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
	cgwName := contentGatewayName(cliName)
	archiveName := fmt.Sprintf("%s_%s_%s.tar.gz", cgwName, runtime.GOOS, runtime.GOARCH)
	link := fmt.Sprintf("%s/%s", strings.TrimRight(cgwURL, "/"), archiveName)

	logrus.Info("Getting binary '", cliName, "' from content gateway: ", link)

	tmp, err := os.MkdirTemp("", cliName)
	if err != nil {
		return "", err
	}

	if err = support.DownloadAndUntarArchive(ctx, link, tmp); err != nil {
		return "", err
	}

	candidates := []string{
		cliName,
		fmt.Sprintf("%s_%s_%s", cgwName, runtime.GOOS, runtime.GOARCH),
		fmt.Sprintf("%s-%s-%s", cliName, runtime.GOOS, runtime.GOARCH),
	}
	if runtime.GOOS == "windows" {
		for i, name := range candidates {
			candidates[i] = name + ".exe"
		}
	}

	for _, name := range candidates {
		path := filepath.Join(tmp, name)
		if _, err = os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("binary for '%s' not found in extracted archive from %s (tried %v)", cliName, link, candidates)
}
