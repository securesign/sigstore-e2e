package testsupport

import (
	"os/exec"

	"github.com/sirupsen/logrus"
)

// IsWSLAvailable checks if WSL is available on the system.
func IsWSLAvailable() bool {
	cmd := exec.Command("wsl", "--version")
	if err := cmd.Run(); err != nil {
		logrus.Warn("WSL is not available on this system: ", err)
		return false
	}
	return true
}
