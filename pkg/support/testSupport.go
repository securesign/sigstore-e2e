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
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/docker/docker/api/types"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/securesign/sigstore-e2e/pkg/api"
)

func DockerAuth() (string, error) {
	authConfig := types.AuthConfig{
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

func Untar(reader io.Reader, writer io.Writer) error {
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
