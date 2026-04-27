package clients

// Blank imports register CLI strategies at init time.
// Remove a line to exclude that strategy and its dependency tree from the binary.
import (
	_ "github.com/securesign/sigstore-e2e/pkg/strategy/cgw"
	_ "github.com/securesign/sigstore-e2e/pkg/strategy/cliserver"
	_ "github.com/securesign/sigstore-e2e/pkg/strategy/container"
	_ "github.com/securesign/sigstore-e2e/pkg/strategy/git"
	_ "github.com/securesign/sigstore-e2e/pkg/strategy/local"
	_ "github.com/securesign/sigstore-e2e/pkg/strategy/openshift"
)
