package strategy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/securesign/sigstore-e2e/pkg/support"
	"github.com/sirupsen/logrus"
)

// Strategy resolves a CLI binary by name and returns its executable path.
type Strategy func(ctx context.Context, cliName string) (string, error)

// Factory creates a Strategy instance, reading its own configuration (env vars, etc.).
type Factory func() Strategy

var (
	mu       sync.Mutex
	registry = map[string]Factory{}
)

func Register(name string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("strategy %q already registered", name))
	}
	registry[name] = f
}

func Get(name string) (Strategy, bool) {
	mu.Lock()
	f, ok := registry[name]
	mu.Unlock()
	if !ok {
		return nil, false
	}
	return f(), true
}

func Has(name string) bool {
	mu.Lock()
	_, ok := registry[name]
	mu.Unlock()
	return ok
}

// DownloadFromLink downloads a .gz file from link, gunzips it, and writes the
// result as an executable named cliName into a temp directory.
func DownloadFromLink(ctx context.Context, cliName string, link string) (string, error) {
	tmp, err := os.MkdirTemp("", cliName)
	if err != nil {
		return "", err
	}

	logrus.Info("Downloading ", cliName, " from ", link)

	var fileName string
	if runtime.GOOS == "windows" {
		fileName = filepath.Join(tmp, cliName+".exe")
	} else {
		fileName = filepath.Join(tmp, cliName)
	}
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0711) //nolint:mnd,gosec
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck

	if err = support.DownloadAndUnzip(ctx, link, file); err != nil {
		return "", err
	}

	return file.Name(), err
}
