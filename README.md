# Sigstore End to End tests
Aim of this test suite is to cover Sigstore deployment by E2E tests. Tests are mostly specific to OpenShift deployment.

## Usage
### Test prerequisites
The test suite relies on a few pre-installed prerequisites. You can install it on your own or with using our [Makefile](./scripts/Makefile).

#### Skopeo
This suite uses Skopeo, which can be installed on different operating systems as follows:

- macOS: Install via Homebrew
```bash
brew install skopeo
```

- Linux:
- For Fedora/CentOS/RHEL:
```bash
sudo dnf install skopeo
```
- For Ubuntu/Debian:
```bash
sudo apt install skopeo
```

- Windows: Skopeo needs to be installed on Windows Subsystem for Linux (WSL). Follow these steps:
1. Install WSL if you haven't already
2. Open your WSL terminal
3. Install Skopeo using the appropriate command for your WSL distribution (see Linux instructions above)

#### Environment Variables
There are scripts provided to set the necessary environment variables:

- For Linux and macOS: tas-env-variables.sh
- For Windows Command Prompt: tas-env-variables.bat
- For Windows PowerShell: tas-env-variables.ps1

Run the appropriate script for your system before running the tests.  
Additionally, you can set the CLI_STRATEGY environment variable to either "openshift" or "local" (default). When set to "openshift", the test suite will download `cosign`, `gitsign`, `rekor-cli`, and `ec` binaries from the cluster's console. For example:
```bash
export CLI_STRATEGY=openshift
```

#### Trusted artifact signer
You can install it by yourself following instructions on https://github.com/securesign/sigstore-ocp/tree/main, via OperatorHub, or by executing
```bash
make install-tas
```
#### Openshift pipelines
You can install the operator from OperatorHub or by using make command.
```bash
make install-tekton
```

### Test
There are tests that require some sort of specific configuration (GitHub token, etc.)
Those tests are automatically skipped if required configuration is not fulfilled.

To automatically install all prerequisites and execute tests perform following command in the root directory.
```bash
make all
```

If the cluster is already prepared and only the tests needs to be started, use
```bash
make test
```
If you want to replace the existing `tas-env-variables.sh` script with the one from https://github.com/securesign/sigstore-ocp/blob/main/tas-env-variables.sh, use:
```bash
make get-env test
```
If you use this command, the file with environment variables needs to be generated only once, unless the cluster or its component paths change.

If you don't have `make` installed or you are on Windows, you can run the tests directly using:
```bash
go test -v ./test/... --ginkgo.v
```
Note that when running tests directly with `go test`, you'll need to ensure all required environment variables are set, including CLI_STRATEGY if you want to use the OpenShift console binaries.

#### Ginkgo
The test suite uses [ginkgo framework](https://onsi.github.io/ginkgo/). You can run the test suite manually by either `go test` command or using the [ginkgo](https://onsi.github.io/ginkgo/#installing-ginkgo) client.
If you decide to do so, you need to set ENV variables defined in [values.go](pkg/api/values.go).

### Cleanup
There is also make target for cluster cleanup.
```bash
make cleanup
```
