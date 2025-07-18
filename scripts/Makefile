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
	go test -v ./test/...

setup:
	@echo "Installing Playwright dependencies..."
	@go run github.com/playwright-community/playwright-go/cmd/playwright install


.PHONY: build test setup all 
 