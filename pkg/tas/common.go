package tas

import (
	"github.com/go-git/go-git/v5"
	"os"
)

const (
	RESOURCES_REPOSITORY = "https://github.com/securesign/sigstore-ocp.git"
)

var repoDir string

func init() {
	var err error
	repoDir, err = Clone(RESOURCES_REPOSITORY)
	if err != nil {
		panic(err)
	}
}

func Clone(url string) (string, error) {
	dir, err := os.MkdirTemp("", "sigstoreClone")
	if err != nil {
		return "", err
	}
	_, err = git.PlainClone(dir, false, &git.CloneOptions{
		URL: url,
	})
	return dir, err
}
