package clients

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"sigstore-e2e-test/pkg/kubernetes"
	"sigstore-e2e-test/pkg/support"

	"github.com/sirupsen/logrus"
)

type cli struct {
	Name      string
	pathToCLI string
	setup     SetupStrategy
}

type SetupStrategy func(context.Context, *cli) (string, error)

func (c *cli) Command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.pathToCLI, args...) // #nosec G204 - we don't expect the code to be running on PROD ENV

	cmd.Stdout = logrus.NewEntry(logrus.StandardLogger()).WithField("app", c.Name).WriterLevel(logrus.InfoLevel)
	cmd.Stderr = logrus.NewEntry(logrus.StandardLogger()).WithField("app", c.Name).WriterLevel(logrus.ErrorLevel)

	return cmd
}

func (c *cli) Setup(ctx context.Context) error {
	var err error
	c.pathToCLI, err = c.setup(ctx, c)
	return err
}

func (c *cli) Destroy(_ context.Context) error {
	return nil
}

func BuildFromGit(url string, branch string) SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		dir, _, err := support.GitClone(url, branch)
		if err != nil {
			return "", err
		}
		err = exec.Command("go", "build", "-C", dir, "-o", c.Name, "./cmd/"+c.Name).Run()
		return dir + "/" + c.Name, err
	}

}

func DownloadFromOpenshift(consoleCliDownloadName string) SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		// Get http link
		link, err := kubernetes.ConsoleCLIDownload(ctx, consoleCliDownloadName, runtime.GOOS)
		if err != nil {
			return "", err
		}

		tmp, err := os.MkdirTemp("", consoleCliDownloadName)
		if err != nil {
			return "", err
		}

		logrus.Info("Downloading ", consoleCliDownloadName, " from ", link)
		fileName := tmp + string(os.PathSeparator) + consoleCliDownloadName
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
		return exec.LookPath(c.Name)
	}

}

func ExtractFromContainer(image string) SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		return "", errors.New("not implemented")
	}
}
