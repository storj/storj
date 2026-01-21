GO_VERSION ?= 1.24.7
NODE_VERSION ?= 24.11.1

GOPATH ?= $(shell go env GOPATH)
BRANCH_NAME ?= $(shell git rev-parse --abbrev-ref HEAD | sed "s!/!-!g")
GIT_TAG := $(shell git rev-parse --short HEAD)
ifeq (${BRANCH_NAME},main)
TAG := ${GIT_TAG}
LATEST_TAG := latest
else
TAG := ${GIT_TAG}-${BRANCH_NAME}
ifneq (,$(findstring release-,$(BRANCH_NAME)))
LATEST_TAG := ${BRANCH_NAME}-latest
endif
endif
CUSTOMTAG ?=


##@ Dependencies

.PHONY: build-dev-deps
build-dev-deps: ## Install dependencies for builds
	go get golang.org/x/tools/cover
	go get github.com/josephspurrier/goversioninfo/cmd/goversioninfo
	go get github.com/github-release/github-release

.PHONY: build-packages
build-packages: build-packages-race build-packages-normal build-satellite-npm build-storagenode-npm build-multinode-npm build-satellite-admin-npm ## Test docker images locally
build-packages-race:
	go build -v ./...
build-packages-normal:
	go build -v -race ./...
build-satellite-npm:
	cd web/satellite && npm ci
build-storagenode-npm:
	cd web/storagenode && npm ci
build-multinode-npm:
	cd web/multinode && npm ci
build-satellite-admin-npm:
	cd satellite/admin/ui && npm ci
	# Temporary until the new back-office replaces the current admin API & UI
	cd satellite/admin/legacy/ui && npm ci

##@ Build

.PHONY: storagenode-console
storagenode-console:
	# build web assets
	rm -rf web/storagenode/dist
	# install npm dependencies and build the binaries
	docker run --rm -i \
		--mount type=bind,src="${PWD}",dst=/go/src/storj.io/storj \
		-w /go/src/storj.io/storj/web/storagenode \
		-e HOME=/tmp \
		-u $(shell id -u):$(shell id -g) \
		node:${NODE_VERSION} \
	  /bin/bash -c "npm ci && npm run build"

.PHONY: multinode-console
multinode-console:
	# build web assets
	rm -rf web/multinode/dist
	# install npm dependencies and build the binaries
	docker run --rm -i \
		--mount type=bind,src="${PWD}",dst=/go/src/storj.io/storj \
		-w /go/src/storj.io/storj/web/multinode \
		-e HOME=/tmp \
		-u $(shell id -u):$(shell id -g) \
		node:${NODE_VERSION} \
	  /bin/bash -c "npm ci && npm run build"

.PHONY: satellite-admin-ui
satellite-admin-ui:
	# install npm dependencies for being embedded by Go embed.
	docker run --rm -i \
		--mount type=bind,src="${PWD}",dst=/go/src/storj.io/storj \
		-w /go/src/storj.io/storj/satellite/admin/ui \
		-e HOME=/tmp \
		-u $(shell id -u):$(shell id -g) \
		node:${NODE_VERSION} \
	  /bin/bash -c "npm ci && npm run build"
	# Temporary until the new back-office replaces the current admin API & UI
	docker run --rm -i \
		--mount type=bind,src="${PWD}",dst=/go/src/storj.io/storj \
		-w /go/src/storj.io/storj/satellite/admin/legacy/ui \
		-e HOME=/tmp \
		-u $(shell id -u):$(shell id -g) \
		node:${NODE_VERSION} \
	  /bin/bash -c "npm ci && npm run build"

##@ Images

.PHONY: images/build
images/build: ## Build images for important components
	docker bake -f release-binaries.docker-bake.hcl -f images.docker-bake.hcl images

.PHONY: images/push
images/push: ## Push Docker images to Docker Hub (jenkins)
	docker bake -f release-binaries.docker-bake.hcl -f images.docker-bake.hcl images --push

.PHONY: images/clean
images/clean: ## Remove all images
	-docker rmi storjlabs/segment-verify:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/jobq:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/multinode:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/satellite:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/versioncontrol:${TAG}${CUSTOMTAG}
	-docker rmi img.dev.storj.io/dev/storagenode:${TAG}${CUSTOMTAG}

##@ Clean

.PHONY: clean
clean: release/binaries/clean images/clean ## Clean docker test environment, local release binaries, and local Docker images
