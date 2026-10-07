package clients

type Tufcli struct {
	*cli
}

// NewTufcli resolves tufcli through the configured strategy, like every other
// CLI. It deliberately has no private fallback: a silent fallback hides the
// case where the release under test does not publish a usable binary, which is
// exactly what the e2e suite exists to catch. Use CLI_STRATEGY=goinstall to
// build from source on purpose.
func NewTufcli() *Tufcli {
	return &Tufcli{
		&cli{
			Name:           "tufcli",
			setupStrategy:  PreferredSetupStrategy(),
			versionCommand: "--version",
		}}
}
