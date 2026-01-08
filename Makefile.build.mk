GO_VERSION ?= 1.24.7
NODE_VERSION ?= 24.11.1

GOPATH ?= $(shell go env GOPATH)
COMPOSE_PROJECT_NAME := ${TAG}-$(shell git rev-parse --abbrev-ref HEAD)
BRANCH_NAME ?= $(shell git rev-parse --abbrev-ref HEAD | sed "s!/!-!g")
GIT_TAG := $(shell git rev-parse --short HEAD)
ifeq (${BRANCH_NAME},main)
TAG    := ${GIT_TAG}
TRACKED_BRANCH := true
LATEST_TAG := latest
else
TAG    := ${GIT_TAG}-${BRANCH_NAME}
ifneq (,$(findstring release-,$(BRANCH_NAME)))
TRACKED_BRANCH := true
LATEST_TAG := ${BRANCH_NAME}-latest
endif
endif
CUSTOMTAG ?=

DOCKER_BUILD := docker build \
	--build-arg TAG=${TAG}

DOCKER_BUILDX := docker buildx build

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

.PHONY: images
images: segment-verify-image jobq-image multinode-image satellite-image uplink-image versioncontrol-image storagenode-image ## Build jobq, multinode, satellite and versioncontrol Docker images
	echo Built version: ${TAG}

.PHONY: segment-verify-image
segment-verify-image: segment-verify_linux_amd64 ## Build segment-verify Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/segment-verify:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/tools/segment-verify/Dockerfile .

.PHONY: jobq-image
jobq-image: jobq_linux_arm jobq_linux_arm64 jobq_linux_amd64 ## Build jobq Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/jobq:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/jobq/Dockerfile .

.PHONY: multinode-image
multinode-image: multinode_linux_arm multinode_linux_arm64 multinode_linux_amd64 ## Build multinode Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/multinode:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/multinode/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/multinode:${TAG}${CUSTOMTAG}-arm32v5 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=arm32v5 \
		-f cmd/multinode/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/multinode:${TAG}${CUSTOMTAG}-arm64v8 \
		--build-arg=GOARCH=arm64 --build-arg=DOCKER_ARCH=arm64v8 \
		-f cmd/multinode/Dockerfile .

.PHONY: uplink-image
uplink-image: uplink_linux_arm uplink_linux_arm64 uplink_linux_amd64 ## Build uplink-cli Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/uplink:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/uplink/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/uplink:${TAG}${CUSTOMTAG}-arm32v5 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=arm32v5 \
		-f cmd/uplink/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/uplink:${TAG}${CUSTOMTAG}-arm64v8 \
		--build-arg=GOARCH=arm64 --build-arg=DOCKER_ARCH=arm64v8 \
		-f cmd/uplink/Dockerfile .

# THIS IS NOT THE PRODUCTION STORAGENODE!!! Only for testing.
# See https://github.com/storj/storagenode-docker for the prod image.
.PHONY: storagenode-image
storagenode-image: storagenode_linux_amd64 identity_linux_amd64
	${DOCKER_BUILD} --pull=true -t img.dev.storj.io/dev/storagenode:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/storagenode/Dockerfile.dev .

.PHONY: satellite-image
satellite-image: satellite_linux_arm satellite_linux_arm64 satellite_linux_amd64 ## Build satellite Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/satellite:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/satellite/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/satellite:${TAG}${CUSTOMTAG}-arm32v5 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=arm32v5 \
		-f cmd/satellite/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/satellite:${TAG}${CUSTOMTAG}-arm64v8 \
		--build-arg=GOARCH=arm64 --build-arg=DOCKER_ARCH=arm64v8 \
		-f cmd/satellite/Dockerfile .

.PHONY: versioncontrol-image
versioncontrol-image: versioncontrol_linux_arm versioncontrol_linux_arm64 versioncontrol_linux_amd64 ## Build versioncontrol Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/versioncontrol:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/versioncontrol/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/versioncontrol:${TAG}${CUSTOMTAG}-arm32v5 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=arm32v5 \
		-f cmd/versioncontrol/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/versioncontrol:${TAG}${CUSTOMTAG}-arm64v8 \
		--build-arg=GOARCH=arm64 --build-arg=DOCKER_ARCH=arm64v8 \
		-f cmd/versioncontrol/Dockerfile .

##@ Deploy

.PHONY: push-images
push-images: ## Push Docker images to Docker Hub (jenkins)
	# images have to be pushed before a manifest can be created
	set -x; for c in multinode satellite uplink versioncontrol ; do \
		docker push storjlabs/$$c:${TAG}${CUSTOMTAG}-amd64 \
		&& docker push storjlabs/$$c:${TAG}${CUSTOMTAG}-arm32v5 \
		&& docker push storjlabs/$$c:${TAG}${CUSTOMTAG}-arm64v8 \
		&& for t in ${TAG}${CUSTOMTAG} ${LATEST_TAG}; do \
			docker manifest create --amend storjlabs/$$c:$$t \
			storjlabs/$$c:${TAG}${CUSTOMTAG}-amd64 \
			storjlabs/$$c:${TAG}${CUSTOMTAG}-arm32v5 \
			storjlabs/$$c:${TAG}${CUSTOMTAG}-arm64v8 \
			&& docker manifest annotate storjlabs/$$c:$$t storjlabs/$$c:${TAG}${CUSTOMTAG}-amd64 --os linux --arch amd64 \
			&& docker manifest annotate storjlabs/$$c:$$t storjlabs/$$c:${TAG}${CUSTOMTAG}-arm32v5 --os linux --arch arm --variant v5 \
			&& docker manifest annotate storjlabs/$$c:$$t storjlabs/$$c:${TAG}${CUSTOMTAG}-arm64v8 --os linux --arch arm64 --variant v8 \
			&& docker manifest push --purge storjlabs/$$c:$$t \
		; done \
	; done

	set -x; for c in segment-verify jobq ; do \
		docker push storjlabs/$$c:${TAG}${CUSTOMTAG}-amd64 \
		&& for t in ${TAG}${CUSTOMTAG} ${LATEST_TAG}; do \
			docker manifest create --amend storjlabs/$$c:$$t \
			storjlabs/$$c:${TAG}${CUSTOMTAG}-amd64 \
			&& docker manifest annotate storjlabs/$$c:$$t storjlabs/$$c:${TAG}${CUSTOMTAG}-amd64 --os linux --arch amd64 \
			&& docker manifest push --purge storjlabs/$$c:$$t \
		; done \
	; done

	docker push img.dev.storj.io/dev/storagenode:${TAG}${CUSTOMTAG}-amd64

##@ Clean

.PHONY: clean
clean: release/binaries/clean clean-images ## Clean docker test environment, local release binaries, and local Docker images

.PHONY: clean-images
clean-images: ## Remove all images
	-docker rmi storjlabs/segment-verify:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/jobq:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/multinode:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/satellite:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/versioncontrol:${TAG}${CUSTOMTAG}
	-docker rmi img.dev.storj.io/dev/storagenode:${TAG}${CUSTOMTAG}-amd64
