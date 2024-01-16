package clients

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/kubernetes"
	"github.com/securesign/sigstore-e2e/pkg/support"
	"github.com/sirupsen/logrus"
)

func PreferredSetupStrategy() SetupStrategy {
	var preferredStrategy SetupStrategy
	switch api.GetValueFor(api.CliStrategy) {
	case "openshift":
		preferredStrategy = DownloadFromOpenshift()
	case "local":
		preferredStrategy = LocalBinary()
	case "openshiftorlocal":
		preferredStrategy = OpenshiftOrLocalBinary()
	default:
		preferredStrategy = LocalBinary()
	}
	return preferredStrategy
}

// Get binaries from Openshift Console if Openshift cluster is used
// or use local binary for all other cases
func OpenshiftOrLocalBinary() SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		isOpenshift, err := kubernetes.IsOpenShift()
		if err != nil {
			return "", err
		}
		if isOpenshift {
			logrus.Info("Cluster detected as Openshift - getting binary from it")
			return DownloadFromOpenshift()(ctx, c)
		} else {
			logrus.Info("Cluster detected, but not Openshift - using local binary instead")
			return LocalBinary()(ctx, c)
		}
	}
}

func BuildFromGit(url string, branch string, buildingDirectory string) SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		logrus.Info("Building '", c.Name, "' from git: ", url, ", branch ", branch)
		dir, _, err := support.GitClone(url, branch)
		if err != nil {
			return "", err
		}
		cmd := exec.Command("go", "build", "-C", dir, "-o", c.Name, buildingDirectory)
		cmd.Stdout = logrus.NewEntry(logrus.StandardLogger()).WithField("app", c.Name).WriterLevel(logrus.InfoLevel)
		cmd.Stderr = logrus.NewEntry(logrus.StandardLogger()).WithField("app", c.Name).WriterLevel(logrus.ErrorLevel)
		err = cmd.Run()

		return dir + "/" + c.Name, err
	}
}

func DownloadFromOpenshift() SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		logrus.Info("Getting binary '", c.Name, "' from Openshift")
		// Get http link
		link, err := kubernetes.ConsoleCLIDownload(ctx, c.Name, runtime.GOOS)
		if err != nil {
			return "", err
		}

		tmp, err := os.MkdirTemp("", c.Name)
		if err != nil {
			return "", err
		}

		logrus.Info("Downloading ", c.Name, " from ", link)
		fileName := tmp + string(os.PathSeparator) + c.Name
		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0711)
		if err != nil {
			return "", err
		}
		defer file.Close()

		if err = support.DownloadAndUnzip(ctx, link, file); err != nil {
			return "", err
		}

		return file.Name(), err
	}
}

func LocalBinary() SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		logrus.Info("Checking local binary '", c.Name, "'")
		return exec.LookPath(c.Name)
	}
}

func ExtractFromContainer(image string, path string) SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return "", err
		}

		registryAuth, err := support.DockerAuth()
		if err != nil {
			return "", err
		}
		pull, err := dockerCli.ImagePull(ctx, image, types.ImagePullOptions{RegistryAuth: registryAuth})
		if err != nil {
			return "", err
		}
		defer pull.Close()
		out := logrus.NewEntry(logrus.StandardLogger()).WithField("app", "docker").WriterLevel(logrus.DebugLevel)
		_, _ = io.Copy(out, pull)

		var cont container.ContainerCreateCreatedBody
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

		defer tarOut.Close()

		cliName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		tmp, err := os.MkdirTemp("", cliName)
		if err != nil {
			return "", err
		}
		fileName := tmp + string(os.PathSeparator) + cliName
		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0711)
		if err != nil {
			return "", err
		}
		defer file.Close()

		r, w := io.Pipe()
		defer r.Close()

		go func() {
			defer w.Close()
			if err := support.UntarFile(tarOut, w); err != nil {
				panic(err)

			}
		}()

		if err = support.Gunzip(r, file); err != nil {
			return "", err
		}
		return file.Name(), err
	}
}
