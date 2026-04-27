package container

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/docker/docker/api/types/container"
	imageDocker "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/securesign/sigstore-e2e/pkg/strategy/testutil"
)

type mockDocker struct {
	pullFn   func(ctx context.Context, ref string, options imageDocker.PullOptions) (io.ReadCloser, error)
	createFn func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	copyFn   func(ctx context.Context, containerID, srcPath string) (io.ReadCloser, container.PathStat, error)
}

func (m *mockDocker) ImagePull(ctx context.Context, ref string, options imageDocker.PullOptions) (io.ReadCloser, error) {
	return m.pullFn(ctx, ref, options)
}

func (m *mockDocker) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
	return m.createFn(ctx, config, hostConfig, networkingConfig, platform, containerName)
}

func (m *mockDocker) CopyFromContainer(ctx context.Context, containerID, srcPath string) (io.ReadCloser, container.PathStat, error) {
	return m.copyFn(ctx, containerID, srcPath)
}

func newMock(tarred []byte) *mockDocker {
	return &mockDocker{
		pullFn: func(_ context.Context, _ string, _ imageDocker.PullOptions) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(nil)), nil
		},
		createFn: func(_ context.Context, _ *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *v1.Platform, _ string) (container.CreateResponse, error) {
			return container.CreateResponse{ID: "test-container-123"}, nil
		},
		copyFn: func(_ context.Context, _, _ string) (io.ReadCloser, container.PathStat, error) {
			return io.NopCloser(bytes.NewReader(tarred)), container.PathStat{}, nil
		},
	}
}

func TestRegistered(t *testing.T) {
	if !strategy.Has("container") {
		t.Fatal("container strategy not registered")
	}
}

func TestStrategy(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho hello\n")
	gzipped := testutil.GzipBytes(t, binaryContent)
	tarred := testutil.TarBytes(t, "tool.gz", gzipped)

	mock := newMock(tarred)

	path, err := extractWithClient(t.Context(), mock, "registry.example.com/image:latest", "/usr/bin/tool.gz", imageDocker.PullOptions{})
	if err != nil {
		t.Fatalf("extractWithClient failed: %v", err)
	}

	testutil.VerifyBinary(t, path, binaryContent)
	t.Logf("OK: tool -> %s", path)
}

func TestStrategyError(t *testing.T) {
	mock := &mockDocker{
		pullFn: func(_ context.Context, _ string, _ imageDocker.PullOptions) (io.ReadCloser, error) {
			return nil, errors.New("pull failed: image not found")
		},
		createFn: func(_ context.Context, _ *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *v1.Platform, _ string) (container.CreateResponse, error) {
			return container.CreateResponse{}, nil
		},
		copyFn: func(_ context.Context, _, _ string) (io.ReadCloser, container.PathStat, error) {
			return nil, container.PathStat{}, nil
		},
	}

	_, err := extractWithClient(t.Context(), mock, "registry.example.com/bad:latest", "/usr/bin/tool.gz", imageDocker.PullOptions{})
	if err == nil {
		t.Fatal("expected error when image pull fails")
	}
}
