package clients

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker/api/types/container"
	imageDocker "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/kubernetes"
	"github.com/securesign/sigstore-e2e/pkg/support"
	"github.com/sirupsen/logrus"
)

var ErrNotFound = errors.New("executable file not found in WSL")

func PreferredSetupStrategy() SetupStrategy {
	var preferredStrategy SetupStrategy
	switch api.GetValueFor(api.CliStrategy) {
	case "openshift":
		preferredStrategy = DownloadFromOpenshift()
	case "cli_server":
		server := api.GetValueFor(api.CLIServerURL)
		if server == "" {
			panic("CLI server URL not specified")
		}
		preferredStrategy = DownloadFromCLIServer(server)
	case "local":
		preferredStrategy = LocalBinary()
	default:
		preferredStrategy = LocalBinary()
	}
	return preferredStrategy
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
		link, err := kubernetes.ConsoleCLIDownload(ctx, c.Name, runtime.GOOS, runtime.GOARCH)
		if err != nil {
			return "", err
		}

		return downloadFromLink(ctx, c, link)
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
		pull, err := dockerCli.ImagePull(ctx, image, imageDocker.PullOptions{RegistryAuth: registryAuth})
		if err != nil {
			return "", err
		}
		defer pull.Close()
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

		defer tarOut.Close()

		cliName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		tmp, err := os.MkdirTemp("", cliName)
		if err != nil {
			return "", err
		}
		fileName := tmp + string(os.PathSeparator) + cliName
		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0711) //nolint:mnd
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

func DownloadFromCLIServer(serverURL string) SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		logrus.Info("Getting binary '", c.Name, "' from CLI server", "Server URL", serverURL)
		link := fmt.Sprintf("https://%s/clients/%s/%s-%s.gz", serverURL, runtime.GOOS, c.Name, runtime.GOARCH)
		return downloadFromLink(ctx, c, link)
	}
}

func downloadFromLink(ctx context.Context, c *cli, link string) (string, error) {
	tmp, err := os.MkdirTemp("", c.Name)
	if err != nil {
		return "", err
	}

	logrus.Info("Downloading ", c.Name, " from ", link)

	var fileName string
	if runtime.GOOS == "windows" {
		fileName = filepath.Join(tmp, c.Name+".exe")
	} else {
		fileName = filepath.Join(tmp, c.Name)
	}
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0711) //nolint:mnd
	if err != nil {
		return "", err
	}
	defer file.Close()

	if err = support.DownloadAndUnzip(ctx, link, file); err != nil {
		return "", err
	}

	return file.Name(), err
}
