package clients

import "runtime"

type Tuftool struct {
	*cli
}

func NewTuftool() *Tuftool {
	var setupStrategy SetupStrategy
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			setupStrategy = PreferredSetupStrategy()
		default:
			setupStrategy = LocalBinary()
		}
	default:
		setupStrategy = LocalBinary()
	}

	return &Tuftool{
		&cli{
			Name:           "tuftool",
			setupStrategy:  setupStrategy,
			versionCommand: "--version",
		}}
}
