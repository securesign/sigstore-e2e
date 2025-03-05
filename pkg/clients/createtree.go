package clients

type CreateTree struct {
	*cli
}

func NewCreateTree() *CreateTree {
	return &CreateTree{
		&cli{
			Name:           "createtree",
			setupStrategy:  PreferredSetupStrategy(),
			versionCommand: "version",
		}}
}
