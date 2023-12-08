BRANCH ?= main
REPO_URL ?= "https://github.com/securesign/sigstore-ocp.git"
WORKDIR := $(shell mktemp -d)

SHELL=/bin/bash

all: get-tas install-tas install-tekton test
gen-env: get-tas

cleanup:
	helm uninstall trusted-artifact-signer -n trusted-artifact-signer || true
	$(shell r=$(REPO_URL); oc delete -k "$${r%.*}/keycloak/resources/base?ref=$(BRANCH)" || true)
	oc delete project fulcio-system rekor-system keycloak-system || true

get-tas:
	@git clone $(REPO_URL) -b $(BRANCH) $(WORKDIR)
	@cd $(WORKDIR)/tas-installer &&	go run main.go envgen
	@cp $(WORKDIR)/tas-installer/tas-env-variables.sh .

install-tas:
	#TODO: involve pull secret
	@cd $(WORKDIR) && printf "rh\njdoe@redhat.com\npassword" > vals && ./tas-easy-install.sh < vals
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
