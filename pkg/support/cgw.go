package support

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var cgwNameOverride = map[string]string{
	"gitsign":   "gitsign_cli",
	"rekor-cli": "rekor_cli",
}

// ContentGatewayName maps a CLI tool name to its content gateway archive name.
func ContentGatewayName(name string) string {
	if override, ok := cgwNameOverride[name]; ok {
		return override
	}
	return strings.ReplaceAll(name, "-", "_")
}

// FindBinary searches for a CLI binary in the given directory using candidate name patterns.
func FindBinary(dir, cliName, goos, goarch string) (string, error) {
	cgwName := ContentGatewayName(cliName)
	candidates := []string{
		cliName,
		fmt.Sprintf("%s_%s_%s", cgwName, goos, goarch),
		fmt.Sprintf("%s-%s-%s", cliName, goos, goarch),
	}
	if goos == "windows" {
		for i, name := range candidates {
			candidates[i] = name + ".exe"
		}
	}

	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("binary for '%s' not found in extracted archive (tried %v)", cliName, candidates)
}
