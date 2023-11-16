package support

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"os"
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
