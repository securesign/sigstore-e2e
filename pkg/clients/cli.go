package clients

import (
	"context"
	"os/exec"

	"github.com/sirupsen/logrus"
)

type cli struct {
	Name           string
	pathToCLI      string
	setupStrategy  SetupStrategy
	versionCommand string
}

type SetupStrategy func(context.Context, *cli) (string, error)

func (c *cli) Command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.pathToCLI, args...) // #nosec G204 - we don't expect the code to be running on PROD ENV

	cmd.Stdout = logrus.NewEntry(logrus.StandardLogger()).WithField("app", c.Name).WriterLevel(logrus.InfoLevel)
	cmd.Stderr = logrus.NewEntry(logrus.StandardLogger()).WithField("app", c.Name).WriterLevel(logrus.ErrorLevel)

	return cmd
}

func (c *cli) CommandOutput(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.pathToCLI, args...) // #nosec G204 - we don't expect the code to be running on PROD ENV
	return cmd.Output()
}

func (c *cli) WithSetupStrategy(strategy SetupStrategy) *cli {
	c.setupStrategy = strategy
	return c
}

func (c *cli) Setup(ctx context.Context) error {
	var err error
	c.pathToCLI, err = c.setupStrategy(ctx, c)
	if err == nil {
		if c.versionCommand != "" {
			logrus.Info("Done. Using '", c.pathToCLI, "' with version:")
			_ = c.Command(ctx, c.versionCommand).Run()
		}
	} else {
		logrus.Error("Failed due to\n   ", err)
	}
	return err
}

func (c *cli) Destroy(_ context.Context) error {
	return nil
}
