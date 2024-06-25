# Sigstore End-to-End Tests

## Table of Contents
- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Running Tests](#running-tests)
- [Notes](#notes)
  
## Overview
This test suite aims to cover Sigstore deployment with end-to-end (E2E) tests, primarily focused on OpenShift deployment.

## Prerequisites
### Required Tools

- **[Skopeo](https://github.com/containers/skopeo):** Used for container image operations.  
- **Trusted Artifact Signer (TAS):** For secure signing capabilities.  
- **OpenShift Pipelines:** To support CI/CD workflows.  

### Environment Setup

- Set environment variables using provided scripts:

  - Linux/macOS: `tas-env-variables.sh`
  - Windows Command Prompt: `tas-env-variables.bat`
  - Windows PowerShell: `tas-env-variables.ps1`


- Optional: Set `CLI_STRATEGY` environment variable to either `openshift` or `local`:
```
export CLI_STRATEGY=openshift
```
This configures the test suite to download `cosign`, `gitsign`, `rekor-cli`, and `ec` binaries from the cluster's console. If not set, the suite will use local binaries by default.

## Installation
### Skopeo

#### macOS
Install via Homebrew:
```
brew install skopeo
```

#### Linux
- Fedora/CentOS/RHEL:  
```
sudo dnf install skopeo
```
- Ubuntu/Debian:  
```
sudo apt install skopeo
```

#### Windows
- Install via Windows Subsystem for Linux (WSL) (see Linux instructions above)


#### Trusted Artifact Signer (TAS)
Options:

1. Follow instructions at https://github.com/securesign/sigstore-ocp/tree/main
2. Install from OperatorHub
3. Use provided Makefile: `make install-tas`

#### OpenShift Pipelines
Options:

1. Install from OperatorHub
2. Use provided Makefile: `make install-tekton`

## Running Tests
### Full Setup and Test Execution
```
make all
```

### Run Tests on Prepared Cluster
```
make test
```
This command will source `tas-env-variables.sh` and run the test suite

### Update Environment Variables and Run Tests
```
make get-env test
```
This command will replace the existing `tas-env-variables.sh` script with the one from https://github.com/securesign/sigstore-ocp/blob/main/tas-env-variables.sh. If you use this command, the file with environment variables needs to be generated whenever the component paths change.

### Manual Test Execution with Ginkgo
You can also run the tests using `go test` command or using the [ginkgo](https://onsi.github.io/ginkgo/#installing-ginkgo) client.
If you decide to do so, you need to set [ENV variables](#environment-setup)
```
source tas-env-variables.sh && go test -v ./test/... --ginkgo.v
```
To run tests in specific directories:
```
ginkgo -v test/cosign test/gitsign
```

### Cleanup
To clean up the cluster after testing:
```
make cleanup
```

## Notes

- Some tests may require specific configurations (e.g., GitHub token) and will be skipped if not fulfilled.
- The test suite uses the [Ginkgo framework](https://onsi.github.io/ginkgo/).
- Environment variables are defined in [values.go](pkg/api/values.go).
