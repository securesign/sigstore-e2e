package clients

type Tuftool struct {
	*cli
}

func NewTuftool() *Tuftool {
	return &Tuftool{
		&cli{
			Name:           "tuftool",
			setupStrategy:  PreferredSetupStrategy(),
			versionCommand: "--version",
		}}
}
