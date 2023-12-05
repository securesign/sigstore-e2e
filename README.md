# Sigstore End to End tests
Aim of this test suite is to cover Sigstore deployment by E2E tests. Tests are mostly specific to OpenShift deployment.

## Usage
### Test prerequisites
The test suite count on a few pre-installed prerequisites. You can install it on your own or with using our [Makefile](./scripts/Makefile).
#### Trusted artifact signer
 You can install it by yourself following instructions on https://github.com/securesign/sigstore-ocp/tree/release-1.0.beta or by executing
```
make install-tas
```
#### Openshift pipelines
You can install the operator from OperatorHub or by using make command.
```
make install-tekton
```

### Test
There are tests that require some sort of specific configuration (GitHub token, etc.)
Those tests are automatically skipped if required configuration is not fulfilled.

To automatically install all prerequisites and execute tests perform following command in the root directory.
```
make all
```

If the cluster is already prepared and anly the tests needs to be started, use
```
make test
```
or, with setting up also the the file with environment variables (`tas-env-variables.sh`)
```
make get-env test
```
File with environment variables needs to be genereated only once, until the cluster or its components paths are not changed.

#### Ginkgo
The test suite uses [ginkgo framework](https://onsi.github.io/ginkgo/). You can run the test suite manually by either `go test` command or using the [ginkgo](https://onsi.github.io/ginkgo/#installing-ginkgo) client.
If you decide to do so, you need to set ENV variables defined in [values.go](pkg/api/values.go).

### Cleanup
There is also make target for cluster cleanup.
```
make cleanup
```
