package support

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/registry"

	"github.com/sirupsen/logrus"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/securesign/sigstore-e2e/pkg/api"
)

func DockerAuth() (string, error) {
	authConfig := registry.AuthConfig{
		Username:      api.GetValueFor(api.DockerRegistryUsername),
		Password:      api.GetValueFor(api.DockerRegistryPassword),
		ServerAddress: "redhat.registry.io",
	}
	if encodedJSON, err := json.Marshal(authConfig); err != nil {
		return "", err
	} else {
		return base64.URLEncoding.EncodeToString(encodedJSON), nil
	}
}

func GitClone(url string, branch string) (string, *git.Repository, error) {
	dir, err := os.MkdirTemp("", "sigstore")
	logrus.Info("Temporary folder created: ", dir)
	if err != nil {
		return "", nil, err
	}
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:           url,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
	})
	return dir, repo, err
}

func GitCloneWithAuth(url string, auth transport.AuthMethod) (string, *git.Repository, error) {
	dir, err := os.MkdirTemp("", "sigstore")
	if err != nil {
		return "", nil, err
	}
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:  url,
		Auth: auth,
	})
	return dir, repo, err
}

func DownloadAndUnzip(ctx context.Context, link string, writer io.Writer) error {
	pr, pw := io.Pipe()
	defer pr.Close()

	go func() {
		_, err := Download(ctx, link, pw)
		pw.CloseWithError(err)
	}()
	return Gunzip(pr, writer)
}

func DownloadAndUntarArchive(ctx context.Context, link string, dst string) error {
	pr, pw := io.Pipe()
	defer pr.Close()

	go func() {
		_, err := Download(ctx, link, pw)
		pw.CloseWithError(err)
	}()
	return UntarArchive(dst, pr)
}

func Download(ctx context.Context, link string, writer io.Writer) (int64, error) {
	client := &http.Client{Timeout: 2 * time.Minute} //nolint:mnd

	const maxRetries = 5
	var lastErr error
	for attempt := range maxRetries {
		if attempt > 0 {
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			logrus.Infof("Retrying download (%d/%d) after %v: %s", attempt+1, maxRetries, delay, link)
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
		if err != nil {
			return 0, err
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("bad status: %s", resp.Status)
			continue
		}
		n, copyErr := io.Copy(writer, resp.Body)
		resp.Body.Close()
		if copyErr != nil {
			lastErr = copyErr
			continue
		}
		return n, nil
	}
	return 0, fmt.Errorf("download failed after %d attempts: %w", maxRetries, lastErr)
}

func ResolveCDNLink(ctx context.Context, link string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return "", fmt.Errorf("expected redirect, got %s", resp.Status)
	}
	location := resp.Header.Get("Location")
	parsed, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("failed to parse redirect location: %w", err)
	}
	cdnURL := parsed.Query().Get("tcDownloadURL")
	if cdnURL == "" {
		return "", fmt.Errorf("tcDownloadURL not found in redirect location")
	}
	return cdnURL, nil
}

func Gunzip(reader io.Reader, writer io.Writer) error {
	gzreader, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer gzreader.Close()

	_, err = io.Copy(writer, gzreader) // #nosec G110 - PROD CLIs are not decompression bomb
	return err
}

func UntarFile(reader io.Reader, writer io.Writer) error {
	tr := tar.NewReader(reader)
	var hdr *tar.Header
	var err error
	if hdr, err = tr.Next(); err != nil {
		return err
	}
	_, err = io.Copy(writer, tr) // #nosec G110 - PROD CLIs are not decompression bomb
	logrus.Debug("untar file from docker image " + hdr.Name)
	return err
}

func UntarArchive(dst string, r io.Reader) error {

	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		clean := filepath.Clean(header.Name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
			return fmt.Errorf("tar entry %q contains path traversal", header.Name)
		}
		target := filepath.Join(dst, clean) // #nosec G305

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil { //nolint:mnd
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			if _, err := os.Stat(filepath.Dir(target)); err != nil {
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil { //nolint:mnd
					return err
				}
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil { //nolint:gosec
				return err
			} // #nosec G110 - Don't expect decompression bomb

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}
