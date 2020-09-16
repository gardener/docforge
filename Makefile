# Copyright 2018 The Gardener Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

REGISTRY                                                 := eu.gcr.io/gardener-project
DOCODE_IMAGE_REPOSITORY                          		 := $(REGISTRY)/docode
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
release: build build-local docker-images docker-login docker-push

.PHONY: docker-images
docker-images:
	@if ! test -f bin/rel/docode; then \
		echo "No docode binary found in bin/rel. Please run 'make build'"; false;\
	fi
	@docker build -t $(DOCODE_IMAGE_REPOSITORY):$(IMAGE_TAG) -t $(DOCODE_IMAGE_REPOSITORY):latest -f Dockerfile --rm .

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

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
	@echo "mode: set" > docode.coverprofile && find . -name "*.coverprofile" -type f | xargs cat | grep -v mode: | sort -r | awk '{if($$1 != last) {print $$0;last=$$1}}' >> docode.coverprofile
	@go tool cover -html=docode.coverprofile -o=docode.coverage.html
	@rm docode.coverprofile

.PHONY: test-clean
test-clean:
	@find . -name "*.coverprofile" -type f -delete
	@rm -f docode.coverage.html
