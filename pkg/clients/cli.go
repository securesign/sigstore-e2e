package clients

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"runtime"
	"sigstore-e2e-test/pkg/kubernetes"
	"sigstore-e2e-test/pkg/support"
)

type cli struct {
	ctx       context.Context
	Name      string
	pathToCLI string
	setup     SetupStrategy
}

type SetupStrategy func(*cli) (string, error)

func (c *cli) Command(args ...string) *exec.Cmd {
	cmd := exec.CommandContext(c.ctx, c.pathToCLI, args...)

	cmd.Stdout = logrus.NewEntry(logrus.StandardLogger()).WithField("app", c.Name).WriterLevel(logrus.InfoLevel)
	cmd.Stderr = logrus.NewEntry(logrus.StandardLogger()).WithField("app", c.Name).WriterLevel(logrus.ErrorLevel)

	return cmd
}

func (c *cli) Setup() error {
	var err error
	c.pathToCLI, err = c.setup(c)
	return err
}

func (c *cli) Destroy() error {
	return nil
}

func BuildFromGit(url string, branch string) SetupStrategy {
	return func(c *cli) (string, error) {
		dir, _, err := support.GitClone(url, branch)
		if err != nil {
			return "", err
		}
		err = exec.Command("go", "build", "-C", dir, "-o", c.Name, "./cmd/"+c.Name).Run()
		return dir + "/" + c.Name, err
	}

}

func DownloadFromOpenshift(consoleCliDownloadName string) SetupStrategy {
	return func(c *cli) (string, error) {
		// Get http link
		link, err := kubernetes.ConsoleCLIDownload(c.ctx, consoleCliDownloadName, runtime.GOOS)
		if err != nil {
			return "", err
		}

		tmp, err := os.MkdirTemp("", consoleCliDownloadName)
		if err != nil {
			return "", err
		}

		logrus.Info("Downloading ", consoleCliDownloadName, " from ", link)
		fileName := tmp + string(os.PathSeparator) + consoleCliDownloadName
		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 711)
		if err != nil {
			return "", err
		}
		defer file.Close()

		if err = support.DownloadAndUnzip(link, file); err != nil {
			return "", err
		}

		return file.Name(), err
	}

}

func LocalBinary() SetupStrategy {
	return func(c *cli) (string, error) {
		return exec.LookPath(c.Name)
	}

}

func ExtractFromContainer(image string) SetupStrategy {
	return func(c *cli) (string, error) {
		return "", errors.New("Not implemented!")
	}
}
