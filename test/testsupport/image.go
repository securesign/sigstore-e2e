package testsupport

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	testImageRegistry = "quay.io/securesign/e2e-tests"
	baseTestImage     = "registry.k8s.io/pause:3.9"
)

// PushTestImage pulls a base image, adds a quay.io auto-expiration label,
// and pushes it to quay.io/securesign/e2e-tests with a unique tag.
// Auth is resolved from the Docker config (~/.docker/config.json).
func PushTestImage(ctx context.Context) (string, error) {
	tag := uuid.New().String()
	targetRef := testImageRegistry + ":" + tag

	srcRef, err := name.ParseReference(baseTestImage)
	if err != nil {
		return "", fmt.Errorf("parsing source reference: %w", err)
	}

	logrus.Infof("Pulling base image: %s", baseTestImage)
	img, err := remote.Image(srcRef, remote.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("pulling image: %w", err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		return "", fmt.Errorf("reading image config: %w", err)
	}
	cfgCopy := *cfg
	if cfgCopy.Config.Labels == nil {
		cfgCopy.Config.Labels = map[string]string{}
	}
	cfgCopy.Config.Labels["quay.expires-after"] = "1h"
	cfgCopy.Config.Labels["run-id"] = tag

	img, err = mutate.ConfigFile(img, &cfgCopy)
	if err != nil {
		return "", fmt.Errorf("setting expiration label: %w", err)
	}

	dstRef, err := name.ParseReference(targetRef)
	if err != nil {
		return "", fmt.Errorf("parsing target reference: %w", err)
	}

	logrus.Infof("Pushing test image: %s", targetRef)
	if err := remote.Write(dstRef, img, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx)); err != nil {
		return "", fmt.Errorf("pushing image: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("computing image digest: %w", err)
	}

	digestRef := testImageRegistry + "@" + digest.String()
	logrus.Infof("Pushed test image: %s (digest: %s)", targetRef, digestRef)
	return digestRef, nil
}
