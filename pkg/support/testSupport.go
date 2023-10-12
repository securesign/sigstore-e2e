package support

import (
	"sigstore-e2e-test/pkg/client"
)

type TestPrerequisite interface {
	Install(c client.Client) error
	Destroy(c client.Client) error
}
