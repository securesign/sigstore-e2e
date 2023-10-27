package support

import (
	"context"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"k8s.io/apimachinery/pkg/util/wait"
	"os"
	"sigstore-e2e-test/pkg/api"
	"time"
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

func WaitUntilIsReady(ctx context.Context, interval time.Duration, timeout time.Duration, components ...api.Readiness) error {
	var err error
	condition := func(ctx context.Context) (bool, error) {
		for _, component := range components {
			if ready, err := component.IsReady(); !ready {
				return false, err
			}
		}
		return true, nil
	}

	err = wait.PollUntilContextTimeout(ctx, interval, timeout, true, condition)
	return err
}
