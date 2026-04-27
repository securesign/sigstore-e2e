package container

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker/api/types/container"
	imageDocker "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/securesign/sigstore-e2e/pkg/support"
	"github.com/sirupsen/logrus"
)

type dockerAPI interface {
	ImagePull(ctx context.Context, ref string, options imageDocker.PullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	CopyFromContainer(ctx context.Context, containerID, srcPath string) (io.ReadCloser, container.PathStat, error)
}

func init() {
	strategy.Register("container", func() strategy.Strategy {
		image := api.GetValueFor(api.ContainerImage)
		if image == "" {
			panic("Container image (CONTAINER_IMAGE) not specified")
		}
		path := api.GetValueFor(api.ContainerPath)
		if path == "" {
			panic("Container path (CONTAINER_PATH) not specified")
		}
		return func(ctx context.Context, cliName string) (string, error) {
			return download(ctx, image, path)
		}
	})
}

func download(ctx context.Context, image string, path string) (string, error) {
	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}

	registryAuth, err := support.DockerAuth()
	if err != nil {
		return "", err
	}
	return extractWithClient(ctx, dockerCli, image, path, imageDocker.PullOptions{RegistryAuth: registryAuth})
}

func extractWithClient(ctx context.Context, dockerCli dockerAPI, image string, path string, pullOpts imageDocker.PullOptions) (string, error) {
	pull, err := dockerCli.ImagePull(ctx, image, pullOpts)
	if err != nil {
		return "", err
	}
	defer pull.Close() //nolint:errcheck
	out := logrus.NewEntry(logrus.StandardLogger()).WithField("app", "docker").WriterLevel(logrus.DebugLevel)
	_, _ = io.Copy(out, pull)

	var cont container.CreateResponse
	if cont, err = dockerCli.ContainerCreate(ctx, &container.Config{Image: image},
		nil,
		nil,
		&v1.Platform{OS: runtime.GOOS},
		uuid.New().String()); err != nil {
		return "", err
	}

	var tarOut io.ReadCloser
	if tarOut, _, err = dockerCli.CopyFromContainer(ctx, cont.ID, path); err != nil {
		return "", err
	}

	defer tarOut.Close() //nolint:errcheck

	binName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	tmp, err := os.MkdirTemp("", binName)
	if err != nil {
		return "", err
	}
	fileName := tmp + string(os.PathSeparator) + binName
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0711) //nolint:mnd,gosec
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck

	r, w := io.Pipe()
	defer r.Close() //nolint:errcheck

	go func() {
		defer w.Close() //nolint:errcheck
		if err := support.UntarFile(tarOut, w); err != nil {
			panic(err)
		}
	}()

	if err = support.Gunzip(r, file); err != nil {
		return "", err
	}
	return file.Name(), err
}
