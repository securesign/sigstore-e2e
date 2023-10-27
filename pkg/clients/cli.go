package clients

import (
	"context"
	"os"
	"os/exec"
	"sigstore-e2e-test/pkg/support"
)

type cli struct {
	ctx       context.Context
	Name      string
	pathToCLI string
	gitUrl    string
	gitBranch string
}

func (c *cli) Command(args ...string) *exec.Cmd {
	cmd := exec.CommandContext(c.ctx, c.pathToCLI, args...)
	cmd.Stderr = os.Stdout
	cmd.Stdout = os.Stdout
	return cmd
}

func (c *cli) Setup() error {
	var err error
	c.pathToCLI, err = exec.LookPath(c.Name)
	if c.pathToCLI == "" {
		c.pathToCLI, err = buildComponent(c)
	}
	return err
}

func (c *cli) Destroy() error {
	return nil
}

func buildComponent(client *cli) (string, error) {
	dir, _, err := support.GitClone(client.gitUrl, client.gitBranch)
	if err != nil {
		return "", err
	}

	err = exec.Command("go", "build", "-C", dir, "-o", client.Name, "./cmd/"+client.Name).Run()
	return dir + "/" + client.Name, err
}
