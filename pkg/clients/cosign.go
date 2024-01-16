package clients

type Cosign struct {
	*cli
}

func NewCosign() *Cosign {
	return &Cosign{
		&cli{
			Name:           "cosign",
			setupStrategy:  PreferredSetupStrategy(),
			versionCommand: "version",
		}}
}
