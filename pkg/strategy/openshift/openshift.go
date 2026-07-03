package openshift

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/securesign/sigstore-e2e/pkg/kubernetes"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/securesign/sigstore-e2e/pkg/support"
	"github.com/sirupsen/logrus"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
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

	if strings.HasSuffix(link, ".tar.gz") {
		return downloadTarGz(ctx, cliName, link)
	}
	return strategy.DownloadFromLink(ctx, cliName, link)
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

	cgwName := contentGatewayName(cliName)
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
