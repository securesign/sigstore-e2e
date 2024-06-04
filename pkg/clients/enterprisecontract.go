package clients

type EnterpriseContract struct {
	*cli
}

func NewEnterpriseContract() *EnterpriseContract {
	return &EnterpriseContract{
		&cli{
			Name:           "ec",
			setupStrategy:  PreferredSetupStrategy(),
			versionCommand: "version",
		}}
}
