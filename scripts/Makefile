BRANCH ?= release-1.0.beta
REPO_URL ?= "https://github.com/securesign/sigstore-ocp.git"
WORKDIR := $(shell mktemp -d)

SHELL=/bin/bash

all: install-tas install-tekton test

cleanup:
	helm uninstall trusted-artifact-signer -n trusted-artifact-signer || true
	$(shell r=$(REPO_URL); oc delete -k "$${r%.*}/keycloak/resources/base?ref=$(BRANCH)" || true)
	oc delete project fulcio-system rekor-system keycloak-system || true

install-tas:
	@git clone $(REPO_URL) -b $(BRANCH) $(WORKDIR)
	#TODO: involve pull secret
	@cd $(WORKDIR) && printf "rh\njdoe@redhat.com\npassword" > vals && ./tas-easy-install.sh < vals
	@cp $(WORKDIR)/tas-env-variables.sh $(shell pwd)/tas-env-variables.sh
	@oc wait job --for=condition=complete copy-secrets-job -n tuf-system --timeout 10m
	@oc wait --for=condition=ready pod -A -l app.kubernetes.io/instance=trusted-artifact-signer --timeout 5m

install-tekton:
	oc apply -f ./scripts/tekton_subscription.yaml
	@until [ ! -z "$$(oc get pod -l app=openshift-pipelines-operator -n openshift-operators 2>/dev/null)" ]; do \
	sleep 10; \
	done
	@oc wait --for=condition=ready pod -l app=openshift-pipelines-operator -n openshift-operators --timeout 1m

test:
	source ./tas-env-variables.sh && go test -v ./test/... --ginkgo.v

.PHONY: install-tas install_tekton test all cleanup
