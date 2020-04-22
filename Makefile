GO_VERSION ?= 1.13.8
GOOS ?= linux
GOARCH ?= amd64
GOPATH ?= $(shell go env GOPATH)
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
	go get golang.org/x/tools/cover
	go get github.com/go-bindata/go-bindata/go-bindata
	go get github.com/josephspurrier/goversioninfo/cmd/goversioninfo

.PHONY: lint
lint: ## Analyze and find programs in source code
	@echo "Running ${@}"
	@golangci-lint run

.PHONY: goimports-fix
goimports-fix: ## Applies goimports to every go file (excluding vendored files)
	goimports -w -local storj.io $$(find . -type f -name '*.go' -not -path "*/vendor/*")

.PHONY: goimports-st
goimports-st: ## Applies goimports to every go file in `git status` (ignores untracked files)
	@git status --porcelain -uno|grep .go|grep -v "^D"|sed -E 's,\w+\s+(.+->\s+)?,,g'|xargs -I {} goimports -w -local storj.io {}

.PHONY: build-packages
build-packages: build-packages-race build-packages-normal build-satellite-npm build-storagenode-npm ## Test docker images locally
build-packages-race:
	go build -v ./...
build-packages-normal:
	go build -v -race ./...
build-satellite-npm:
	cd web/satellite && npm ci
build-storagenode-npm:
	cd web/storagenode && npm ci

##@ Simulator

.PHONY: install-sim
install-sim: ## install storj-sim
	@echo "Running ${@}"
	go install -race -v \
		storj.io/storj/cmd/satellite \
		storj.io/storj/cmd/storagenode \
		storj.io/storj/cmd/storj-sim \
		storj.io/storj/cmd/versioncontrol \
		storj.io/storj/cmd/uplink \
		storj.io/storj/cmd/identity \
		storj.io/storj/cmd/certificates

	## install exact version of storj/gateway
	mkdir -p .build/gateway-tmp
	-cd .build/gateway-tmp && go mod init gatewaybuild
	cd .build/gateway-tmp && GO111MODULE=on go get storj.io/gateway@v1.0.0-rc.8

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

.PHONY: test-libuplink-gomobile
test-libuplink-gomobile: ## Run gomobile tests
	@./lib/uplink-gomobile/test-sim.sh

.PHONY: check-satellite-config-lock
check-satellite-config-lock: ## Test if the satellite config file has changed (jenkins)
	@echo "Running ${@}"
	@cd scripts; ./check-satellite-config-lock.sh

.PHONY: test-sim-backwards-compatible
test-sim-backwards-compatible: ## Test uploading a file with lastest release (jenkins)
	@echo "Running ${@}"
	@./scripts/test-sim-backwards.sh

.PHONY: check-monitoring
check-monitoring: ## Check for locked monkit calls that have changed
	@echo "Running ${@}"
	@check-monitoring ./... | diff -U0 ./monkit.lock - \
	|| (echo "Locked monkit metrics have been changed. Notify #data-science and run \`go run github.com/storj/ci/check-monitoring -out monkit.lock ./...\` to update monkit.lock file." \
	&& exit 1)

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
		node:10.15.1 \
	  /bin/bash -c "npm ci && npm run build"
	# embed web assets into go
	go-bindata -prefix web/storagenode/ -fs -o storagenode/console/consoleassets/bindata.resource.go -pkg consoleassets web/storagenode/dist/... web/storagenode/static/...
	# configure existing go code to know about the new assets
	/usr/bin/env echo -e '\nfunc init() { FileSystem = AssetFile() }' >> storagenode/console/consoleassets/bindata.resource.go
	gofmt -w -s storagenode/console/consoleassets/bindata.resource.go

.PHONY: images
images: satellite-image segment-reaper-image storagenode-image uplink-image versioncontrol-image ## Build satellite, segment-reaper, storagenode, uplink, and versioncontrol Docker images
	echo Built version: ${TAG}

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

.PHONY: segment-reaper-image
segment-reaper-image: segment-reaper_linux_amd64 segment-reaper_linux_arm segment-reaper_linux_arm64 ## Build segment-reaper Docker image
	${DOCKER_BUILD} --pull=true -t storjlabs/segment-reaper:${TAG}${CUSTOMTAG}-amd64 \
		-f cmd/segment-reaper/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/segment-reaper:${TAG}${CUSTOMTAG}-arm32v6 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=arm32v6 \
		-f cmd/segment-reaper/Dockerfile .
	${DOCKER_BUILD} --pull=true -t storjlabs/segment-reaper:${TAG}${CUSTOMTAG}-aarch64 \
		--build-arg=GOARCH=arm --build-arg=DOCKER_ARCH=aarch64 \
		-f cmd/segment-reaper/Dockerfile .

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
	[ "${GOOS}" = "windows" ] && [ "${GOARCH}" = "amd64" ] && goversioninfo $$sixtyfour -o cmd/${COMPONENT}/resource.syso \
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
	storj.io/storj/cmd/${COMPONENT}
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

.PHONY: certificates_%
certificates_%:
	$(MAKE) binary-check COMPONENT=certificates GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: identity_%
identity_%:
	$(MAKE) binary-check COMPONENT=identity GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: inspector_%
inspector_%:
	$(MAKE) binary-check COMPONENT=inspector GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: satellite_%
satellite_%:
	$(MAKE) binary-check COMPONENT=satellite GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: segment-reaper_%
segment-reaper_%:
	$(MAKE) binary-check COMPONENT=segment-reaper GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: storagenode_%
storagenode_%: storagenode-console
	$(MAKE) binary-check COMPONENT=storagenode GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: storagenode-updater_%
storagenode-updater_%:
	$(MAKE) binary-check COMPONENT=storagenode-updater GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: uplink_%
uplink_%:
	$(MAKE) binary-check COMPONENT=uplink GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))
.PHONY: versioncontrol_%
versioncontrol_%:
	$(MAKE) binary-check COMPONENT=versioncontrol GOARCH=$(word 3, $(subst _, ,$@)) GOOS=$(word 2, $(subst _, ,$@))


COMPONENTLIST := certificates identity inspector satellite storagenode storagenode-updater uplink versioncontrol
OSARCHLIST    := darwin_amd64 linux_amd64 linux_arm linux_arm64 windows_amd64 freebsd_amd64
BINARIES      := $(foreach C,$(COMPONENTLIST),$(foreach O,$(OSARCHLIST),$C_$O))
.PHONY: binaries
binaries: ${BINARIES} ## Build certificates, identity, inspector, satellite, storagenode, uplink, and versioncontrol binaries (jenkins)

.PHONY: sign-windows-installer
sign-windows-installer:
	storj-sign release/${TAG}/storagenode_windows_amd64.msi

.PHONY: libuplink
libuplink:
	go build -ldflags="-s -w" -buildmode c-shared -o uplink.so storj.io/storj/lib/uplinkc
	cp lib/uplinkc/uplink_definitions.h uplink_definitions.h

.PHONY: libuplink-gomobile
libuplink-gomobile:
	@./lib/uplink-gomobile/build.sh

##@ Deploy

.PHONY: push-images
push-images: ## Push Docker images to Docker Hub (jenkins)
	# images have to be pushed before a manifest can be created
	# satellite
	for c in satellite segment-reaper storagenode uplink versioncontrol ; do \
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
	cd "release/${TAG}"; for f in *; do \
		c="$${f%%_*}" \
		&& if [ "$${f##*.}" != "$${f}" ]; then \
			ln -s "$${f}" "$${f%%_*}.$${f##*.}" \
			&& zip "$${f}.zip" "$${f%%_*}.$${f##*.}" \
			&& rm "$${f%%_*}.$${f##*.}" \
		; else \
			ln -sf "$${f}" "$${f%%_*}" \
			&& zip "$${f}.zip" "$${f%%_*}" \
			&& rm "$${f%%_*}" \
		; fi \
	; done
	cd "release/${TAG}"; gsutil -m cp -r *.zip "gs://storj-v3-alpha-builds/${TAG}/"

##@ Clean

.PHONY: clean
clean: test-docker-clean binaries-clean clean-images ## Clean docker test environment, local release binaries, and local Docker images

.PHONY: binaries-clean
binaries-clean: ## Remove all local release binaries (jenkins)
	rm -rf release

.PHONY: clean-images
clean-images:
	-docker rmi storjlabs/satellite:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/storagenode:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/uplink:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/versioncontrol:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/segment-reaper:${TAG}${CUSTOMTAG}

.PHONY: test-docker-clean
test-docker-clean: ## Clean up Docker environment used in test-docker target
	-docker-compose down --rmi all


##@ Tooling

.PHONY: diagrams
diagrams:
	archview -root "storj.io/storj/satellite.Core"     -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/ ./satellite/... | dot -T svg -o satellite-core.svg
	archview -root "storj.io/storj/satellite.API"      -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/ ./satellite/... | dot -T svg -o satellite-api.svg
	archview -root "storj.io/storj/satellite.Repairer" -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/ ./satellite/... | dot -T svg -o satellite-repair.svg
	archview -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/   ./satellite/...   | dot -T svg -o satellite.svg
	archview -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/storagenode/ ./storagenode/... | dot -T svg -o storage-node.svg

.PHONY: diagrams-graphml
diagrams-graphml:
	archview -root "storj.io/storj/satellite.Core"     -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/ -out satellite-core.graphml   ./satellite/...
	archview -root "storj.io/storj/satellite.API"      -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/ -out satellite-api.graphml    ./satellite/...
	archview -root "storj.io/storj/satellite.Repairer" -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/ -out satellite-repair.graphml ./satellite/...
	archview -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/   -out satellite.graphml    ./satellite/...
	archview -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/storagenode/ -out storage-node.graphml ./storagenode/...

.PHONY: update-satellite-config-lock
update-satellite-config-lock: ## Update the satellite config lock file
	@docker run -ti --rm \
		-v ${GOPATH}/pkg/mod:/go/pkg/mod \
		-v ${CURDIR}:/storj \
		-v $(shell go env GOCACHE):/go-cache \
		-e "GOCACHE=/go-cache" \
		-u root:root \
		golang:${GO_VERSION} \
		/bin/bash -c "cd /storj/scripts; ./update-satellite-config-lock.sh"
