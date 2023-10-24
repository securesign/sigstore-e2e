package tas

import (
	"sigstore-e2e-test/pkg/support"
)

const (
	RESOURCES_REPOSITORY = "https://github.com/securesign/sigstore-ocp.git"
)

var repoDir string

func init() {
	var err error
	repoDir, _, err = support.GitClone(RESOURCES_REPOSITORY)
	if err != nil {
		panic(err)
	}
}
