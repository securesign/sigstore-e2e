package api

import (
	"context"
)

type TestPrerequisite interface {
	Setup(ctx context.Context) error
	Destroy(ctx context.Context) error
}
