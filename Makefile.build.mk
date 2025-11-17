GO_VERSION ?= 1.24.7
GOOS ?= linux
GOARCH ?= amd64
GOPATH ?= $(shell go env GOPATH)
NODE_VERSION ?= 24.11.1
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
SUB_DIR = ""

FILEEXT :=
ifeq (${GOOS},windows)
FILEEXT := .exe
endif

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
	cd satellite/admin/back-office/ui && npm ci

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
		-w /go/src/storj.io/storj/satellite/admin/back-office/ui \
		-e HOME=/tmp \
		-u $(shell id -u):$(shell id -g) \
		node:${NODE_VERSION} \
	  /bin/bash -c "npm ci && npm run build"

.PHONY: satellite-wasm
satellite-wasm:
	docker run --rm -i -v "${PWD}":/go/src/storj.io/storj -e GO111MODULE=on \
	-e GOOS=js -e GOARCH=wasm -e GOARM=6 -e CGO_ENABLED=1 \
	-v /tmp/go-cache:/tmp/.cache/go-build -v /tmp/go-pkg:/go/pkg \
	-w /go/src/storj.io/storj -e GOPROXY -e TAG=${TAG} -u $(shell id -u):$(shell id -g) storjlabs/golang:${GO_VERSION} \
	scripts/build-wasm.sh ;\

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

.PHONY: darwin-binaries
darwin-binaries:
	echo "Building Darwin binaries (identity, uplink) for amd64, arm64"
	@components="identity uplink"; \
	arches="amd64 arm64"; \
	for comp in $$components; do \
		for arch in $$arches; do \
			output_dir="release/$(TAG)"; \
			bin_name="$$comp"_darwin_"$$arch$(FILEEXT)"; \
			dest_path="$$output_dir/$$bin_name"; \
			if [ -f "$$dest_path" ]; then \
				echo "$$dest_path already exists, skipping..."; \
				continue; \
			fi; \
			echo ""; \
			echo "Building $$comp for Darwin/$$arch"; \
			mkdir -p "$$output_dir"; \
			mkdir -p /tmp/go-cache /tmp/go-pkg; \
			\
			docker run --rm -i \
				-v "$$PWD":/go/src/storj.io/storj \
				-e GO111MODULE=on \
				-e GOOS=darwin \
				-e GOARCH="$$arch" \
				-e GOARM=6 \
				-e CGO_ENABLED=1 \
				-e GOCACHE=/tmp/.cache/go-build \
				-e CGO_ENABLED=false \
				-v /tmp/go-cache:/tmp/.cache/go-build \
				-v /tmp/go-pkg:/go/pkg \
				-w /go/src/storj.io/storj \
				-e GOPROXY \
				-u $(shell id -u):$(shell id -g) \
				golang:$(GO_VERSION) \
				scripts/release.sh build $(EXTRA_ARGS) \
					-o "$$dest_path" \
					storj.io/storj/cmd/$$comp; \
			\
			chmod 755 "$$dest_path"; \
			rm -f "$${dest_path%.exe}.zip"; \
		done; \
	done

.PHONY: binary
binary: CUSTOMTAG = -${GOOS}-${GOARCH}
binary:
	@if [ -z "${COMPONENT}" ]; then echo "Try one of the following targets instead:" \
		&& for b in binaries ${BINARIES}; do echo "- $$b"; done && exit 1; fi
	mkdir -p release/${TAG}
	mkdir -p /tmp/go-cache /tmp/go-pkg
	rm -f cmd/${SUB_DIR}${COMPONENT}/resource.syso
	if [ "${GOARCH}" = "amd64" ]; then sixtyfour="-64"; fi; \
	[ "${GOOS}" = "windows" ] && [ "${GOARCH}" = "amd64" ] && goversioninfo $$sixtyfour -o cmd/${SUB_DIR}${COMPONENT}/resource.syso \
	-original-name ${COMPONENT}_${GOOS}_${GOARCH}${FILEEXT} \
	-description "${COMPONENT} program for Storj" \
        -product-ver-major "$(shell git describe --tags --exact-match --match "v[0-9]*\.[0-9]*\.[0-9]*" | awk -F'.' 'BEGIN {v=0} {gsub("v", "", $$0); v=$$1} END {print v}' )" \
                -ver-major "$(shell git describe --tags --exact-match --match "v[0-9]*\.[0-9]*\.[0-9]*" | awk -F'.' 'BEGIN {v=0} {gsub("v", "", $$0); v=$$1} END {print v}' )" \
        -product-ver-minor "$(shell git describe --tags --exact-match --match "v[0-9]*\.[0-9]*\.[0-9]*" | awk -F'.' 'BEGIN {v=0} {v=$$2} END {print v}')" \
                -ver-minor "$(shell git describe --tags --exact-match --match "v[0-9]*\.[0-9]*\.[0-9]*" | awk -F'.' 'BEGIN {v=0} {v=$$2} END {print v}')" \
        -product-ver-patch "$(shell git describe --tags --exact-match --match "v[0-9]*\.[0-9]*\.[0-9]*" | awk -F'.' 'BEGIN {v=0} {v=$$3} END {print v}' | awk -F'-' 'BEGIN {v=0} {v=$$1} END {print v}')" \
                -ver-patch "$(shell git describe --tags --exact-match --match "v[0-9]*\.[0-9]*\.[0-9]*" | awk -F'.' 'BEGIN {v=0} {v=$$3} END {print v}' | awk -F'-' 'BEGIN {v=0} {v=$$1} END {print v}')" \
        -product-version "$(shell git describe --tags --exact-match --match "v[0-9]*\.[0-9]*\.[0-9]*" | awk -F'-' 'BEGIN {v=0} {v=$$1} END {print v}' || echo "dev" )" \
        -special-build "$(shell git describe --tags --exact-match --match "v[0-9]*\.[0-9]*\.[0-9]*" | awk -F'-' 'BEGIN {v=0} {v=$$2} END {print v}' )" \
	resources/versioninfo.json || echo "goversioninfo is not installed, metadata will not be created"
	docker run --rm -i -v "${PWD}":/go/src/storj.io/storj -e GO111MODULE=on \
	-e GOOS=${GOOS} -e GOARCH=${GOARCH} -e GOARM=6 -e CGO_ENABLED=1 \
	-v /tmp/go-cache:/tmp/.cache/go-build -v /tmp/go-pkg:/go/pkg \
	-w /go/src/storj.io/storj -e GOPROXY -u $(shell id -u):$(shell id -g) storjlabs/golang:${GO_VERSION} \
	scripts/release.sh build $(EXTRA_ARGS) -o release/${TAG}/$(COMPONENT)_${GOOS}_${GOARCH}${FILEEXT} \
	storj.io/storj/cmd/${SUB_DIR}${COMPONENT}

	if [ "${COMPONENT}" = "satellite" ] && [ "${GOOS}" = "linux" ] && [ "${GOARCH}" = "amd64" ]; \
	then \
		echo "Building wasm code"; \
		$(MAKE) satellite-wasm; \
	fi

	chmod 755 release/${TAG}/$(COMPONENT)_${GOOS}_${GOARCH}${FILEEXT}
	[ "${FILEEXT}" = ".exe" ] && storj-sign release/${TAG}/$(COMPONENT)_${GOOS}_${GOARCH}${FILEEXT} || echo "Skipping signing"
	rm -f release/${TAG}/${COMPONENT}_${GOOS}_${GOARCH}.zip

.PHONY: binary-check
binary-check:
	@if [ -f release/${TAG}/${COMPONENT}_${GOOS}_${GOARCH} ] || [ -f release/${TAG}/${COMPONENT}_${GOOS}_${GOARCH}.exe ]; \
	then \
		echo "release/${TAG}/${COMPONENT}_${GOOS}_${GOARCH} exists"; \
	else \
		echo "Making ${COMPONENT}"; \
		$(MAKE) binary; \
	fi

.PHONY: segment-verify_%
segment-verify_%:
	$(MAKE) binary-check SUB_DIR="tools/" COMPONENT=segment-verify GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: jobq_%
jobq_%:
	$(MAKE) binary-check COMPONENT=jobq GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: certificates_%
certificates_%:
	$(MAKE) binary-check COMPONENT=certificates GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: identity_%
identity_%:
	$(MAKE) binary-check COMPONENT=identity GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: multinode_%
multinode_%: multinode-console
	$(MAKE) binary-check COMPONENT=multinode GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: satellite_%
satellite_%: satellite-admin-ui
	$(MAKE) binary-check COMPONENT=satellite GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: storagenode_%
storagenode_%: storagenode-console
	$(MAKE) binary-check COMPONENT=storagenode GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: storagenode-updater_%
storagenode-updater_%:
	EXTRA_ARGS="-tags=service" $(MAKE) binary-check COMPONENT=storagenode-updater GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: uplink_%
uplink_%:
	$(MAKE) binary-check COMPONENT=uplink GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: versioncontrol_%
versioncontrol_%:
	$(MAKE) binary-check COMPONENT=versioncontrol GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: multinode_%
multinode_%: multinode-console
	$(MAKE) binary-check COMPONENT=multinode GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))


COMPONENTLIST          := certificates identity multinode satellite storagenode storagenode-updater uplink versioncontrol
COMPONENTLIST_AMD_ONLY := segment-verify jobq
OSARCHLIST             := linux_amd64 linux_arm linux_arm64 windows_amd64 freebsd_amd64
OSARCHLIST_AMD_ONLY    := linux_amd64
BINARIES               := $(foreach C,$(COMPONENTLIST),$(foreach O,$(OSARCHLIST),$C_$O))
BINARIES_AMD_ONLY      := $(foreach C,$(COMPONENTLIST_AMD_ONLY),$(foreach O,$(OSARCHLIST_AMD_ONLY),$C_$O))
.PHONY: binaries
binaries: ${BINARIES} ${BINARIES_AMD_ONLY} ## Build segment-verify jobq certificates, identity, multinode, satellite, storagenode, uplink, versioncontrol and multinode binaries (jenkins)


.PHONY: sign-windows-installer
sign-windows-installer:
	storj-sign release/${TAG}/storagenode_windows_amd64.msi

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

.PHONY: binaries-upload
binaries-upload: ## Upload binaries to Google Storage (jenkins)
	cd "release/${TAG}"; for f in *; do \
		zipname=$$(echo $${f} | sed 's/.exe//g') \
		&& filename=$$(echo $${f} | sed 's/_.*\.exe/.exe/g' | sed 's/_.*\.msi/.msi/g' | sed 's/_.*//g') \
		&& if [ "$${f}" != "$${filename}" ]; then \
			ln $${f} $${filename} \
			&& zip -r "$${zipname}.zip" "$${filename}" \
			&& rm $${filename} \
		; else \
			zip -r "$${zipname}.zip" "$${filename}" \
		; fi \
	; done
	cd "release/${TAG}" \
		&& sha256sum *.zip > sha256sums \
		&& gsutil -m cp -r *.zip sha256sums "gs://storj-v3-alpha-builds/${TAG}/"

.PHONY: publish-release
publish-release:
	scripts/publish-release.sh ${BRANCH_NAME} "release/${TAG}"

##@ Clean

.PHONY: clean
clean: binaries-clean clean-images ## Clean docker test environment, local release binaries, and local Docker images

.PHONY: binaries-clean
binaries-clean: ## Remove all local release binaries (jenkins)
	rm -rf release

.PHONY: clean-images
clean-images: ## Remove all images
	-docker rmi storjlabs/segment-verify:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/jobq:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/multinode:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/satellite:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/versioncontrol:${TAG}${CUSTOMTAG}
	-docker rmi img.dev.storj.io/dev/storagenode:${TAG}${CUSTOMTAG}-amd64
