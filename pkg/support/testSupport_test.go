package support

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func buildZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeZip(t *testing.T, content []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "archive-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestUnzipArchive(t *testing.T) {
	content := []byte("#!/bin/sh\necho hi\n")
	archivePath := writeZip(t, buildZip(t, map[string][]byte{
		"rekor-cli.exe":     content,
		"nested/other-file": []byte("data"),
	}))

	dst := t.TempDir()
	if err := UnzipArchive(dst, archivePath); err != nil {
		t.Fatalf("UnzipArchive failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "rekor-cli.exe")) //nolint:gosec
	if err != nil {
		t.Fatalf("reading extracted binary: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %q, want %q", got, content)
	}

	info, err := os.Stat(filepath.Join(dst, "rekor-cli.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatalf("extracted binary is not executable (mode: %s)", info.Mode())
	}

	if _, err := os.Stat(filepath.Join(dst, "nested", "other-file")); err != nil {
		t.Fatalf("nested file not extracted: %v", err)
	}
}

func TestUnzipArchivePathTraversal(t *testing.T) {
	archivePath := writeZip(t, buildZip(t, map[string][]byte{
		"../../evil": []byte("data"),
	}))

	if err := UnzipArchive(t.TempDir(), archivePath); err == nil {
		t.Fatal("expected path traversal error, got nil")
	}
}
