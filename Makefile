#
# Copyright 2022 The OpenFunction Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

VERSION?=$(shell cat VERSION | tr -d " \t\n\r")
# Image URL to use all building/pushing image targets
IMG ?= openfunction/revision-controller:$(VERSION)
IMG_DEV ?= openfunctiondev/revision-controller:$(VERSION)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
#CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"
CRD_OPTIONS ?= "crd:preserveUnknownFields=false,maxDescLen=0"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development
fmt: goimports ## Run go fmt && goimports against code.
	go fmt ./...
	$(GOIMPORTS) -w -local github.com/openfunction main.go controllers/ pkg/ docs/ hack/ apis/

GOIMPORTS=$(shell pwd)/bin/goimports
goimports:
	$(call go-get-tool,$(GOIMPORTS),golang.org/x/tools/cmd/goimports@v0.1.7)

vet: ## Run go vet against code.
	go vet ./...

##@ Build
docker-build: ## Build docker image with the openfunction.
	docker build -t ${IMG} .

docker-push: ## Push docker image with the openfunction.
	docker push ${IMG}

docker-build-dev: ## Build dev docker image with the openfunction.
	docker build -t ${IMG_DEV} .

docker-push-dev: ## Push dev docker image with the openfunction.
	docker push ${IMG_DEV}

