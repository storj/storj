GO_VERSION ?= 1.13.0
GOOS ?= linux
GOARCH ?= amd64
COMPOSE_PROJECT_NAME := ${TAG}-$(shell git rev-parse --abbrev-ref HEAD)
BRANCH_NAME ?= $(shell git rev-parse --abbrev-ref HEAD | sed "s!/!-!g")
ifeq (${BRANCH_NAME},master)
TAG    := $(shell git rev-parse --short HEAD)-go${GO_VERSION}
TRACKED_BRANCH := true
LATEST_TAG := latest
else
TAG    := $(shell git rev-parse --short HEAD)-${BRANCH_NAME}-go${GO_VERSION}
ifneq (,$(findstring release-,$(BRANCH_NAME)))
TRACKED_BRANCH := true
LATEST_TAG := ${BRANCH_NAME}-latest
endif
endif
CUSTOMTAG ?=

FILEEXT :=
ifeq (${GOOS},windows)
FILEEXT := .exe
endif

DOCKER_BUILD := docker build \
	--build-arg TAG=${TAG}

.DEFAULT_GOAL := help
.PHONY: help
help:
	@awk 'BEGIN { \
		FS = ":.*##"; \
		printf "\nUsage:\n  make \033[36m<target>\033[0m\n"\
	} \
	/^[a-zA-Z_-]+:.*?##/ { \
		printf "  \033[36m%-17s\033[0m %s\n", $$1, $$2 \
	} \
	/^##@/ { \
		printf "\n\033[1m%s\033[0m\n", substr($$0, 5) \
	} ' $(MAKEFILE_LIST)

##@ Dependencies

.PHONY: build-dev-deps
build-dev-deps: ## Install dependencies for builds
	go get github.com/mattn/goveralls
	go get golang.org/x/tools/cover
	go get github.com/modocache/gover
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b ${GOPATH}/bin v1.19.1

.PHONY: lint
lint: check-copyrights ## Analyze and find programs in source code
	@echo "Running ${@}"
	@golangci-lint run

.PHONY: check-copyrights
check-copyrights: ## Check source files for copyright headers
	@echo "Running ${@}"
	@go run ./scripts/check-copyright.go

.PHONY: goimports-fix
goimports-fix: ## Applies goimports to every go file (excluding vendored files)
	goimports -w -local storj.io $$(find . -type f -name '*.go' -not -path "*/vendor/*")

.PHONY: goimports-st
goimports-st: ## Applies goimports to every go file in `git status` (ignores untracked files)
	@git status --porcelain -uno|grep .go|grep -v "^D"|sed -E 's,\w+\s+(.+->\s+)?,,g'|xargs -I {} goimports -w -local storj.io {}

.PHONY: proto
proto: ## Rebuild protobuf files
	@echo "Running ${@}"
	go run scripts/protobuf.go install
	go run scripts/protobuf.go generate

.PHONY: build-packages
build-packages: build-packages-race build-packages-normal build-npm ## Test docker images locally
build-packages-race:
	go build -v ./...
build-packages-normal:
	go build -v -race ./...
build-npm:
	cd web/satellite && npm ci

##@ Simulator

.PHONY: install-sim
install-sim: ## install storj-sim
	@echo "Running ${@}"
	@go install -race -v storj.io/storj/cmd/storj-sim storj.io/storj/cmd/versioncontrol storj.io/storj/cmd/bootstrap storj.io/storj/cmd/satellite storj.io/storj/cmd/storagenode storj.io/storj/cmd/uplink storj.io/storj/cmd/gateway storj.io/storj/cmd/identity storj.io/storj/cmd/certificates

##@ Test

.PHONY: test
test: ## Run tests on source code (jenkins)
	go test -race -v -cover -coverprofile=.coverprofile ./...
	@echo done

.PHONY: test-sim
test-sim: ## Test source with storj-sim (jenkins)
	@echo "Running ${@}"
	@./scripts/test-sim.sh

.PHONY: test-certificates
test-certificates: ## Test certificate signing service and storagenode setup (jenkins)
	@echo "Running ${@}"
	@./scripts/test-certificates.sh

.PHONY: test-docker
test-docker: ## Run tests in Docker
	docker-compose up -d --remove-orphans test
	docker-compose run test make test

.PHONY: check-satellite-config-lock
check-satellite-config-lock: ## Test if the satellite config file has changed (jenkins)
	@echo "Running ${@}"
	@cd scripts; ./check-satellite-config-lock.sh

.PHONY: all-in-one
all-in-one: ## Deploy docker images with one storagenode locally
	export VERSION="${TAG}${CUSTOMTAG}" \
	&& $(MAKE) satellite-image storagenode-image gateway-image \
	&& docker-compose up --scale storagenode=1 satellite gateway

.PHONY: test-all-in-one
test-all-in-one: ## Test docker images locally
	export VERSION="${TAG}${CUSTOMTAG}" \
	&& $(MAKE) satellite-image storagenode-image gateway-image \
	&& ./scripts/test-aio.sh

.PHONY: test-sim-backwards-compatible
test-sim-backwards-compatible: ## Test uploading a file with lastest release (jenkins)
	@echo "Running ${@}"
	@./scripts/test-sim-backwards.sh

##@ Build

.PHONY: images
images: bootstrap-image gateway-image satellite-image storagenode-image uplink-image versioncontrol-image ## Build bootstrap, gateway, satellite, storagenode, uplink, and versioncontrol Docker images
	echo Built version: ${TAG}

.PHONY: bootstrap-image
bootstrap-image: bootstrap_linux_arm bootstrap_linux_arm64 bootstrap_linux_amd64 ## Build bootstrap Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/bootstrap:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/bootstrap/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/bootstrap:${TAG}${CUSTOMTAG}-arm32v6 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=arm32v6 \
		-f cmd/bootstrap/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/bootstrap:${TAG}${CUSTOMTAG}-aarch64 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=aarch64 \
		-f cmd/bootstrap/Dockerfile .
.PHONY: gateway-image
gateway-image: gateway_linux_arm gateway_linux_arm64 gateway_linux_amd64 ## Build gateway Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/gateway:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/gateway/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/gateway:${TAG}${CUSTOMTAG}-arm32v6 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=arm32v6 \
		-f cmd/gateway/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/gateway:${TAG}${CUSTOMTAG}-aarch64 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=aarch64 \
		-f cmd/gateway/Dockerfile .
.PHONY: satellite-image
satellite-image: satellite_linux_arm satellite_linux_arm64 satellite_linux_amd64 ## Build satellite Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/satellite:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/satellite/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/satellite:${TAG}${CUSTOMTAG}-arm32v6 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=arm32v6 \
		-f cmd/satellite/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/satellite:${TAG}${CUSTOMTAG}-aarch64 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=aarch64 \
		-f cmd/satellite/Dockerfile .
.PHONY: storagenode-image
storagenode-image: storagenode_linux_arm storagenode_linux_arm64 storagenode_linux_amd64 ## Build storagenode Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/storagenode:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/storagenode/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/storagenode:${TAG}${CUSTOMTAG}-arm32v6 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=arm32v6 \
		-f cmd/storagenode/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/storagenode:${TAG}${CUSTOMTAG}-aarch64 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=aarch64 \
		-f cmd/storagenode/Dockerfile .
.PHONY: uplink-image
uplink-image: uplink_linux_arm uplink_linux_arm64 uplink_linux_amd64 ## Build uplink Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/uplink:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/uplink/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/uplink:${TAG}${CUSTOMTAG}-arm32v6 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=arm32v6 \
		-f cmd/uplink/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/uplink:${TAG}${CUSTOMTAG}-aarch64 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=aarch64 \
		-f cmd/uplink/Dockerfile .
.PHONY: versioncontrol-image
versioncontrol-image: versioncontrol_linux_arm versioncontrol_linux_arm64 versioncontrol_linux_amd64 ## Build versioncontrol Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/versioncontrol:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/versioncontrol/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/versioncontrol:${TAG}${CUSTOMTAG}-arm32v6 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=arm32v6 \
		-f cmd/versioncontrol/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/versioncontrol:${TAG}${CUSTOMTAG}-aarch64 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=aarch64 \
		-f cmd/versioncontrol/Dockerfile .

.PHONY: binary
binary: CUSTOMTAG = -${GOOS}-${GOARCH}
binary:
	@if [ -z "${COMPONENT}" ]; then echo "Try one of the following targets instead:" \
		&& for b in binaries ${BINARIES}; do echo "- $$b"; done && exit 1; fi
	mkdir -p release/${TAG}
	mkdir -p /tmp/go-cache /tmp/go-pkg
	rm -f cmd/${COMPONENT}/resource.syso
	if [ "${GOARCH}" = "amd64" ]; then sixtyfour="-64"; fi; \
	[ "${GOARCH}" = "amd64" ] && goversioninfo $$sixtyfour -o cmd/${COMPONENT}/resource.syso \
	-original-name ${COMPONENT}_${GOOS}_${GOARCH}${FILEEXT} \
	-description "${COMPONENT} program for Storj" \
	-product-ver-build 9 -ver-build 9 \
	-product-version "alpha9" \
	resources/versioninfo.json || echo "goversioninfo is not installed, metadata will not be created"
	docker run --rm -i -v "${PWD}":/go/src/storj.io/storj -e GO111MODULE=on \
	-e GOOS=${GOOS} -e GOARCH=${GOARCH} -e GOARM=6 -e CGO_ENABLED=1 \
	-v /tmp/go-cache:/tmp/.cache/go-build -v /tmp/go-pkg:/go/pkg \
	-w /go/src/storj.io/storj -e GOPROXY -u $(shell id -u):$(shell id -g) storjlabs/golang:${GO_VERSION} \
	scripts/release.sh build -o release/${TAG}/$(COMPONENT)_${GOOS}_${GOARCH}${FILEEXT} \
	storj.io/storj/cmd/${COMPONENT}
	chmod 755 release/${TAG}/$(COMPONENT)_${GOOS}_${GOARCH}${FILEEXT}
	[ "${FILEEXT}" = ".exe" ] && storj-sign release/${TAG}/$(COMPONENT)_${GOOS}_${GOARCH}${FILEEXT} || echo "Skipping signing"
	rm -f release/${TAG}/${COMPONENT}_${GOOS}_${GOARCH}.zip

.PHONY: bootstrap_%
bootstrap_%:
	GOOS=$(word 2, $(subst _, ,$@)) GOARCH=$(word 3, $(subst _, ,$@)) COMPONENT=bootstrap $(MAKE) binary
	$(MAKE) binary-check COMPONENT=bootstrap GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: gateway_%
gateway_%:
	GOOS=$(word 2, $(subst _, ,$@)) GOARCH=$(word 3, $(subst _, ,$@)) COMPONENT=gateway $(MAKE) binary
	$(MAKE) binary-check COMPONENT=gateway GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: satellite_%
satellite_%:
	GOOS=$(word 2, $(subst _, ,$@)) GOARCH=$(word 3, $(subst _, ,$@)) COMPONENT=satellite $(MAKE) binary
	$(MAKE) binary-check COMPONENT=satellite GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: storagenode_%
storagenode_%:
	$(MAKE) binary-check COMPONENT=storagenode GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: binary-check
binary-check:
	@if [ -f release/${TAG}/${COMPONENT}_${GOOS}_${GOARCH} ]; then echo "release/${TAG}/${COMPONENT}_${GOOS}_${GOARCH} exists"; else $(MAKE) binary; fi
.PHONY: uplink_%
uplink_%:
	GOOS=$(word 2, $(subst _, ,$@)) GOARCH=$(word 3, $(subst _, ,$@)) COMPONENT=uplink $(MAKE) binary
	$(MAKE) binary-check COMPONENT=uplink GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: identity_%
identity_%:
	$(MAKE) binary-check COMPONENT=identity GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: certificates_%
certificates_%:
	GOOS=$(word 2, $(subst _, ,$@)) GOARCH=$(word 3, $(subst _, ,$@)) COMPONENT=certificates $(MAKE) binary
.PHONY: inspector_%
inspector_%:
	GOOS=$(word 2, $(subst _, ,$@)) GOARCH=$(word 3, $(subst _, ,$@)) COMPONENT=inspector $(MAKE) binary
.PHONY: versioncontrol_%
versioncontrol_%:
	GOOS=$(word 2, $(subst _, ,$@)) GOARCH=$(word 3, $(subst _, ,$@)) COMPONENT=versioncontrol $(MAKE) binary
.PHONY: linksharing_%
linksharing_%:
	GOOS=$(word 2, $(subst _, ,$@)) GOARCH=$(word 3, $(subst _, ,$@)) COMPONENT=linksharing $(MAKE) binary
.PHONY: storagenode-updater_%
storagenode-updater_%:
	GOOS=$(word 2, $(subst _, ,$@)) GOARCH=$(word 3, $(subst _, ,$@)) COMPONENT=storagenode-updater $(MAKE) binary	

COMPONENTLIST := bootstrap certificates gateway identity inspector linksharing satellite storagenode storagenode-updater uplink versioncontrol
OSARCHLIST    := darwin_amd64 linux_amd64 linux_arm linux_arm64 windows_amd64
BINARIES      := $(foreach C,$(COMPONENTLIST),$(foreach O,$(OSARCHLIST),$C_$O))
.PHONY: binaries
binaries: ${BINARIES} ## Build bootstrap, certificates, gateway, identity, inspector, linksharing, satellite, storagenode, uplink, and versioncontrol binaries (jenkins)

.PHONY: libuplink
libuplink:
	go build -ldflags="-s -w" -buildmode c-shared -o uplink.so storj.io/storj/lib/uplinkc
	cp lib/uplinkc/uplink_definitions.h uplink_definitions.h

##@ Deploy

.PHONY: deploy
deploy: ## Update Kubernetes deployments in staging (jenkins)
	for deployment in $$(kubectl --context nonprod -n v3 get deployment -l app=storagenode --output=jsonpath='{.items..metadata.name}'); do \
		kubectl --context nonprod --namespace v3 patch deployment $$deployment -p"{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"storagenode\",\"image\":\"storjlabs/storagenode:${TAG}\"}]}}}}" ; \
	done
	kubectl --context nonprod --namespace v3 patch deployment earth-satellite -p"{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"satellite\",\"image\":\"storjlabs/satellite:${TAG}\"}]}}}}"

.PHONY: push-images
push-images: ## Push Docker images to Docker Hub (jenkins)
	# images have to be pushed before a manifest can be created
	# satellite
	for c in bootstrap gateway satellite storagenode uplink versioncontrol ; do \
		docker push storjlabs/$$c:${TAG}${CUSTOMTAG}-amd64 \
		&& docker push storjlabs/$$c:${TAG}${CUSTOMTAG}-arm32v6 \
		&& docker push storjlabs/$$c:${TAG}${CUSTOMTAG}-aarch64 \
		&& for t in ${TAG}${CUSTOMTAG} ${LATEST_TAG}; do \
			docker manifest create storjlabs/$$c:$$t \
			storjlabs/$$c:${TAG}${CUSTOMTAG}-amd64 \
			storjlabs/$$c:${TAG}${CUSTOMTAG}-arm32v6 \
			storjlabs/$$c:${TAG}${CUSTOMTAG}-aarch64 \
			&& docker manifest annotate storjlabs/$$c:$$t storjlabs/$$c:${TAG}${CUSTOMTAG}-amd64 --os linux --arch amd64 \
			&& docker manifest annotate storjlabs/$$c:$$t storjlabs/$$c:${TAG}${CUSTOMTAG}-arm32v6 --os linux --arch arm --variant v6 \
			&& docker manifest annotate storjlabs/$$c:$$t storjlabs/$$c:${TAG}${CUSTOMTAG}-aarch64 --os linux --arch arm64 \
			&& docker manifest push --purge storjlabs/$$c:$$t \
		; done \
	; done

.PHONY: binaries-upload
binaries-upload: ## Upload binaries to Google Storage (jenkins)
	cd "release/${TAG}"; for f in *; do zip $${f}.zip $${f}; done
	cd "release/${TAG}"; gsutil -m cp -r *.zip "gs://storj-v3-alpha-builds/${TAG}/"

##@ Clean

.PHONY: clean
clean: test-docker-clean binaries-clean clean-images ## Clean docker test environment, local release binaries, and local Docker images

.PHONY: binaries-clean
binaries-clean: ## Remove all local release binaries (jenkins)
	rm -rf release

.PHONY: clean-images
clean-images:
	-docker rmi storjlabs/bootstrap:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/gateway:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/satellite:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/storagenode:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/uplink:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/versioncontrol:${TAG}${CUSTOMTAG}

.PHONY: test-docker-clean
test-docker-clean: ## Clean up Docker environment used in test-docker target
	-docker-compose down --rmi all


##@ Tooling

.PHONY: diagrams
diagrams:
	archview -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/   ./satellite/...   | dot -T svg -o satellite.svg
	archview -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/storagenode/ ./storagenode/... | dot -T svg -o storage-node.svg

.PHONY: diagrams-graphml
diagrams-graphml:
	archview -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/   -out satellite.graphml    ./satellite/...
	archview -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/storagenode/ -out storage-node.graphml ./storagenode/...

.PHONY: update-satellite-config-lock
update-satellite-config-lock: ## Update the satellite config lock file
	@docker run -ti --rm \
		-v ${GOPATH}/pkg/mod:/go/pkg/mod \
		-v $(shell pwd):/storj \
		-v $(shell go env GOCACHE):/go-cache \
		-e "GOCACHE=/go-cache" \
		-u root:root \
		golang:${GO_VERSION} \
		/bin/bash -c "cd /storj/scripts; ./update-satellite-config-lock.sh"
