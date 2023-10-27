# Sigstore End to End tests
Aim of this test suite is to cover Sigstore deployment by E2E tests. Tests are mostly specific to OpenShift deployment.

## Usage
In order to run all tests that does not require any special configuration you can use following command:
```
go test -v ./...
```

There are tests that require some sort of specific configuration (GitHub token, etc.)
Those tests are automatically skipped if required configuration is not fulfilled.
### Ginkgo
The test suite uses [ginkgo framework](https://onsi.github.io/ginkgo/). 
Using the [ginkgo](https://onsi.github.io/ginkgo/#installing-ginkgo) client is also possible.
