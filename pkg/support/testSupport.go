package support

import (
	"compress/gzip"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

func GetEnvOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func DownloadAndUnzip(link string) (string, error) {
	client := &http.Client{}
	resp, _ := client.Get(link)
	defer resp.Body.Close()

	file, err := os.CreateTemp("", strings.TrimSuffix(filepath.Base(link), filepath.Ext(filepath.Base(link))))
	defer file.Close()

	if err != nil {
		return "", err
	}

	gzreader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(file, gzreader)
	defer gzreader.Close()
	if err != nil {
		return "", err
	}
	return file.Name(), nil
}
