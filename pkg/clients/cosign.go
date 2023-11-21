package clients

import (
	"context"
)

type Cosign struct {
	*cli
}

func NewCosign(ctx context.Context) *Cosign {
	return &Cosign{
		&cli{
			Name:      "cosign",
			ctx:       ctx,
			gitUrl:    "https://github.com/securesign/cosign",
			gitBranch: "redhat-v2.1.1",
		}}
}
