# Sigstore End-to-End Tests

## Table of Contents
- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Running Tests](#running-tests)
- [Notes](#notes)
  
## Overview
This test suite aims to cover Trusted Artifact Signer deployment with end-to-end (E2E) tests, primarily focused on OpenShift deployment.

## Prerequisites
### Required Tools

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
This configures the test suite to download `cosign`, `gitsign`, `rekor-cli`, `ec`, `tuftool` binaries from the cluster's console. If not set, the suite will use local binaries by default.

- Optional: To use a manual image setup, set the `MANUAL_IMAGE_SETUP` environment variable to `true` and specify the `TARGET_IMAGE_NAME`.
```
export MANUAL_IMAGE_SETUP=true
export TARGET_IMAGE_NAME="ttl.sh/$(uuidgen):10m"
podman push $TARGET_IMAGE_NAME
```

For daemonless runners, you can use tools like [skopeo](https://github.com/containers/skopeo).
```
skopeo copy docker://docker.io/library/alpine:latest docker://$TARGET_IMAGE_NAME
```

## Installation
#### Trusted Artifact Signer (TAS)
Options:

1. Follow instructions at https://github.com/securesign/sigstore-ocp/tree/main
2. Install from OperatorHub

## Running Tests
### Full Setup and Test Execution
```
make all
```

### Load Environment Variables and Run Tests
```
make env test
```

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

## Notes

- Some tests may require specific configurations (e.g., GitHub token) and will be skipped if not fulfilled.
- The test suite uses the [Ginkgo framework](https://onsi.github.io/ginkgo/).
- Environment variables are defined in [values.go](pkg/api/values.go).
