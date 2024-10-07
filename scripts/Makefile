SHELL=/bin/bash

# options: openshift, local
CLI_STRATEGY ?= local

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.54.2
golangci-lint:
	@[ -f $(GOLANGCI_LINT) ] || { \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) $(GOLANGCI_LINT_VERSION) ;\
	}

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

all: build env test

env:
	@oc project trusted-artifact-signer
	@./tas-env-variables.sh > .env

build:
	go build ./...

test:
	@echo "Running tests..."
	@if [ -f .env ]; then \
		echo "Loading environment variables from .env"; \
		set -o allexport && source .env && set +o allexport; \
	else \
		echo ".env file not found, running tests without environment variables"; \
	fi; \
	go test -v ./test/cosign ./test/gitsign ./test/rekorcli --ginkgo.v

tuf-repo:
	@echo "Running manual TUF repository test..."
	@export SIGSTORE_OIDC_ISSUER=$(shell echo "https://$$(oc get route keycloak -n keycloak-system | tail -n 1 | awk '{print $$2}')/auth/realms/trusted-artifact-signer"); \
	go test -v ./test/manual_tuf_repo --ginkgo.v

test-tuf-repo: tuf-repo env test

.PHONY: build test test-tuf-repo all 
 