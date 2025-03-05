package clients

type UpdateTree struct {
	*cli
}

func NewUpdateTree() *UpdateTree {
	return &UpdateTree{
		&cli{
			Name:           "updatetree",
			setupStrategy:  PreferredSetupStrategy(),
			versionCommand: "version",
		}}
}
