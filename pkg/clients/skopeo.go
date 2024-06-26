package clients

import (
	"context"
	"os/exec"

	"github.com/sirupsen/logrus"
)

type Skopeo struct {
	*cli
}

func NewSkopeo() *Skopeo {
	return &Skopeo{
		&cli{
			Name:           "skopeo",
			setupStrategy:  LocalBinary(),
			versionCommand: "--version",
		}}
}

func (c *cli) WSLCommand(ctx context.Context, args ...string) *exec.Cmd {
	wslArgs := append([]string{"skopeo"}, args...)
	cmd := exec.CommandContext(ctx, "wsl", wslArgs...) // #nosec G204 - we don't expect the code to be running on PROD ENV

	cmd.Stdout = logrus.NewEntry(logrus.StandardLogger()).WithField("app", c.Name).WriterLevel(logrus.InfoLevel)
	cmd.Stderr = logrus.NewEntry(logrus.StandardLogger()).WithField("app", c.Name).WriterLevel(logrus.ErrorLevel)

	return cmd
}
