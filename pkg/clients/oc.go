package clients

type Oc struct {
	*cli
}

func NewOc() *Oc {
	return &Oc{
		&cli{
			Name:           "oc",
			setupStrategy:  LocalBinary(),
			versionCommand: "version",
		}}
}
