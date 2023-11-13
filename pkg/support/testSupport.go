package support

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"os"
	"sigstore-e2e-test/pkg/client"
)

type TestPrerequisite interface {
	Install(c client.Client) error
	Destroy(c client.Client) error
	Readiness
}

type Readiness interface {
	IsReady(c client.Client) bool
}

func GitClone(url string) (string, *git.Repository, error) {
	dir, err := os.MkdirTemp("", "sigstore")
	if err != nil {
		return "", nil, err
	}
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL: url,
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
