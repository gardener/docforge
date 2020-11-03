# SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

REGISTRY                                                 := eu.gcr.io/gardener-project
DOCODE_IMAGE_REPOSITORY                          		 := $(REGISTRY)/docforge
IMAGE_TAG                                                := $(shell cat VERSION)

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: update-version
update-version:
	@./hack/update-version

.PHONY: revendor
revendor:
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
verify: check test

.PHONY: check
check:
	@.ci/check

.PHONY: test
test:
	@.ci/test

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
