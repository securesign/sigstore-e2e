package support

import (
	"github.com/go-git/go-git/v5"
	"os"
	"sigstore-e2e-test/pkg/client"
)

type TestPrerequisite interface {
	Install(c client.Client) error
	Destroy(c client.Client) error
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
