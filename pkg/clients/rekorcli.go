package clients

type RekorCli struct {
	*cli
}

func NewRekorCli() *RekorCli {
	return &RekorCli{
		&cli{
			Name:           "rekor-cli",
			setupStrategy:  PreferredSetupStrategy(),
			versionCommand: "version",
		}}
}
