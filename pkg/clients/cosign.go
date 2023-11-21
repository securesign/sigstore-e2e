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
			Name:  "cosign",
			ctx:   ctx,
			setup: DownloadFromOpenshift("cosign"),
		}}
}
