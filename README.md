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


- Optional: Set `CLI_STRATEGY` environment variable to configure how CLI binaries are obtained:
```
export CLI_STRATEGY=openshift
```
  Available strategies:
  - `local` (default) — uses binaries already on `$PATH`
  - `openshift` — downloads from the cluster's `ConsoleCLIDownload` resources (version controlled by `CLI_CGW_VERSION`, defaults to the latest published GA)
  - `cli_server` — downloads from a CLI server (requires `CLI_SERVER_URL`)
  - `cgw` — downloads from the Red Hat content gateway (requires `CGW_URL`)

  This matters because each strategy puts a different build under test:

  | `CLI_STRATEGY` | Binaries under test |
  |---|---|
  | `local` (default) | whatever is on `$PATH` — whatever you built or installed. If `CLI_LOCAL_DIR` is set, resolution is pinned to exactly that directory with no `$PATH` fallback; leaving it unset keeps the `$PATH` search |
  | `openshift` | the published GA release named by `CLI_CGW_VERSION`, fetched via the cluster's `ConsoleCLIDownload` links. Defaults to the latest published GA (see `pkg/api/values.go`); never falls back to a different version, a local binary, or `go install` |
  | `cli_server` | binaries served by `CLI_SERVER_URL` |
  | `cgw` | the published GA release named in `CGW_URL` |
  | `goinstall` | a source build from `GOINSTALL_MODULE` — deliberately not a released artifact |

  A CI pipeline that extracts its own release-candidate binaries onto `$PATH` should set `CLI_LOCAL_DIR` to that extraction directory, so a binary missing from the snapshot fails loudly instead of silently falling back to an unrelated copy found elsewhere on `$PATH`.

  For the `cgw` strategy, set the base URL including the RHTAS version:
```
export CLI_STRATEGY=cgw
# GA
export CGW_URL=https://developers.redhat.com/content-gateway/file/cgw/RHTAS/1.4.0
# Stage
# export CGW_URL=https://developers.qa.redhat.com/content-gateway/file/cgw/RHTAS/1.4.0
```

  `CLI_CGW_VERSION` exists because a cluster running an unreleased operator build
  advertises a `ConsoleCLIDownload` version that the content gateway has not
  published yet, so downloading whatever version the cluster happens to
  advertise is unreliable. With `CLI_STRATEGY=openshift`, the version actually
  used is whatever `CLI_CGW_VERSION` resolves to: if it's unset, it defaults to
  the latest published GA RHTAS version, maintained in `pkg/api/values.go`
  (bump it there when a new z-stream ships); set it explicitly to pin a
  specific version instead. Either way, the resolved version is used verbatim
  — there is no fallback to another version, to a local binary, or to `go
  install`. A failed download fails the run outright rather than silently
  testing the wrong binaries. The `local` and `cgw` strategies do not need it.
  Testing an unpublished release-candidate binary is not this strategy's job —
  use `CLI_STRATEGY=local` with `CLI_LOCAL_DIR` pinned to the RC's extraction
  directory instead (see `securesign/releases`' cross-platform-tests
  workflow).
```
export CLI_STRATEGY=openshift
export CLI_CGW_VERSION=1.5.0
```

  This repo's own E2E workflow (`.github/workflows/e2e.yml`) runs the operator
  under test from `main` (unreleased) while pulling CLI binaries for the
  newest published GA release — the two necessarily differ, since the content
  gateway only ever hosts published GA releases.

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
