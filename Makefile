# SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

REGISTRY                                                 := europe-docker.pkg.dev/gardener-project/releases
DOCODE_IMAGE_REPOSITORY                          		 := $(REGISTRY)/docforge
IMAGE_TAG                                                := $(shell cat VERSION)

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: tidy
tidy:
	@GO111MODULE=on go mod tidy

.PHONY: build
build:
	@.ci/build

.PHONY: build-local
build-local:
	@env LOCAL_BUILD=1 .ci/build

.PHONY: release
release: build build-local docker-image docker-login docker-push

.PHONY: docker-image
docker-image:
	@if ! test -f bin/rel/docforge-linux-amd64; then \
		echo "No docforge-linux-amd64 binary found in bin/rel. Please run 'make build'"; false;\
	fi
	@docker build -t $(DOCODE_IMAGE_REPOSITORY):$(IMAGE_TAG) -t $(DOCODE_IMAGE_REPOSITORY):latest -f Dockerfile --rm .

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-login-local
docker-login-local:
	@gcloud auth


.PHONY: docker-push
docker-push:
	@if ! docker images $(DOCODE_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(IMAGE_TAG); then echo "$(DOCODE_IMAGE_REPOSITORY) version $(IMAGE_TAG) is not built yet. Please run 'make docker-images'"; false; fi
	@docker push $(DOCODE_IMAGE_REPOSITORY):$(IMAGE_TAG)

.PHONY: docker-push-kind
docker-push-kind:
	@kind load docker-image $(DOCODE_IMAGE_REPOSITORY):$(IMAGE_TAG)

.PHONY: clean
clean:
	@rm -rf bin/

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: verify
verify: check test integration-test e2e

.PHONY: check
check:
	@.ci/check

.PHONY: test
test:
	@.ci/test

.PHONY: integration-test
integration-test:
	@.ci/integration-test
	
.PHONY: e2e
e2e:
	@.ci/e2e

.PHONY: test-cov
test-cov:
	@env COVERAGE=1 .ci/test
	@echo "mode: set" > docforge.coverprofile && find . -name "*.coverprofile" -type f | xargs cat | grep -v mode: | sort -r | awk '{if($$1 != last) {print $$0;last=$$1}}' >> docforge.coverprofile
	@go tool cover -html=docforge.coverprofile -o=docforge.coverage.html
	@rm docforge.coverprofile

.PHONY: test-clean
test-clean:
	@find . -name "*.coverprofile" -type f -delete
	@rm -f docforge.coverage.html

.PHONY: docs-gen
docs-gen:
	@.ci/docs-gen

.PHONY: check-manifest
check-manifest:
	@.ci/check-manifest

.PHONY: generate
generate:
	@go generate ./...

.PHONY: task
task:
	@./task.sh "$(m)"

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix


##@ Dependencies
## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

GOLANGCI_LINT_VERSION ?= v1.61.0
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef