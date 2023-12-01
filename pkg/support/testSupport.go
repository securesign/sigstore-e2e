package support

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

func GitClone(url string, branch string) (string, *git.Repository, error) {
	dir, err := os.MkdirTemp("", "sigstore")
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

	go func() {
		defer pw.Close()
		if _, err := Download(ctx, link, pw); err != nil {
			panic(err)
		}

	}()
	return Gunzip(pr, writer)
}

func Download(ctx context.Context, link string, writer io.Writer) (int64, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bad status: %s", resp.Status)
	}
	defer resp.Body.Close()
	return io.Copy(writer, resp.Body)
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
