package clients

import "github.com/securesign/sigstore-e2e/pkg/api"

type Cosign struct {
	*cli
}

func NewCosign() *Cosign {
	return &Cosign{
		&cli{
			Name: "cosign",
			setupStrategies: []SetupStrategy{
				DownloadFromOpenshift(),
				BuildFromGit(api.GetValueFor(api.CosignRepo), api.GetValueFor(api.CosignRepoBranch), "./cmd/cosign"),
				LocalBinary(),
			},
			versionCommand: "version",
		}}
}
