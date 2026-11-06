package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFindBinaryWindowsPreservesExeExtension guards against a regression where the
// symlink created for a renamed candidate (e.g. rekor_cli_windows_amd64.exe -> rekor-cli)
// dropped the .exe suffix, leaving a path that Windows refuses to execute directly.
func TestFindBinaryWindowsPreservesExeExtension(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "rekor_cli_windows_amd64.exe")
	if err := os.WriteFile(actual, []byte("binary"), 0755); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	path, err := FindBinary(dir, "rekor-cli", "windows", "amd64")
	if err != nil {
		t.Fatalf("FindBinary failed: %v", err)
	}

	if !strings.HasSuffix(path, ".exe") {
		t.Fatalf("expected returned path to end in .exe, got %q", path)
	}
	if filepath.Base(path) != "rekor-cli.exe" {
		t.Fatalf("expected returned path basename to be rekor-cli.exe, got %q", filepath.Base(path))
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("returned path %q does not exist: %v", path, err)
	}
}

func TestFindBinaryNonWindowsNoExeSuffix(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "rekor_cli_linux_amd64")
	if err := os.WriteFile(actual, []byte("binary"), 0755); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	path, err := FindBinary(dir, "rekor-cli", "linux", "amd64")
	if err != nil {
		t.Fatalf("FindBinary failed: %v", err)
	}

	if filepath.Base(path) != "rekor-cli" {
		t.Fatalf("expected returned path basename to be rekor-cli, got %q", filepath.Base(path))
	}
}

func TestFindBinaryDirectMatchNoSymlink(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "cosign.exe")
	if err := os.WriteFile(actual, []byte("binary"), 0755); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	path, err := FindBinary(dir, "cosign", "windows", "amd64")
	if err != nil {
		t.Fatalf("FindBinary failed: %v", err)
	}
	if path != actual {
		t.Fatalf("expected direct match %q, got %q", actual, path)
	}
}

func TestContentGatewayLinkForVersion(t *testing.T) {
	tests := []struct {
		name    string
		link    string
		version string
		want    string
		wantErr bool
	}{
		{
			name:    "rewrites plain RHTAS link",
			link:    "https://developers.redhat.com/content-gateway/file/RHTAS/1.5.1/cosign_linux_amd64.tar.gz",
			version: "1.5.0",
			want:    "https://developers.redhat.com/content-gateway/file/RHTAS/1.5.0/cosign_linux_amd64.tar.gz",
		},
		{
			name:    "rewrites cgw/RHTAS link",
			link:    "https://developers.redhat.com/content-gateway/file/cgw/RHTAS/1.4.0/cosign_linux_amd64.tar.gz",
			version: "1.5.0",
			want:    "https://developers.redhat.com/content-gateway/file/cgw/RHTAS/1.5.0/cosign_linux_amd64.tar.gz",
		},
		{
			name:    "errors when link has no RHTAS segment",
			link:    "https://developers.redhat.com/content-gateway/file/cosign_linux_amd64.tar.gz",
			version: "1.5.0",
			wantErr: true,
		},
		{
			name:    "errors on unparseable URL",
			link:    "://not a url",
			version: "1.5.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ContentGatewayLinkForVersion(tt.link, tt.version)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got link %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
