.PHONY: test lint proto check-copyrights build-dev-deps


GO_VERSION ?= 1.11
GOOS ?= linux
GOARCH ?= amd64
COMPOSE_PROJECT_NAME := ${TAG}-$(shell git rev-parse --abbrev-ref HEAD)
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
ifeq (${BRANCH},master)
TAG    	:= $(shell git rev-parse --short HEAD)-go${GO_VERSION}
else
TAG    	:= $(shell git rev-parse --short HEAD)-${BRANCH}-go${GO_VERSION}
endif
CUSTOMTAG ?=

FILEEXT :=
ifeq (${GOOS},windows)
FILEEXT := .exe
endif

DOCKER_BUILD := docker build \
	--build-arg GO_VERSION=${GO_VERSION} \
	--build-arg GOOS=${GOOS} \
	--build-arg GOARCH=${GOARCH}

# currently disabled linters:
#   gofmt               # enable after switch to go1.11
#   goimpor             # enable after switch to go1.11
#   unparam             # enable later
#   gosec               # enable later
#   vetshadow           # enable later
#   gochecknoinits      # enable later
#   gochecknoglobals    # enable later
#   dupl                # needs tuning
#   gocyclo             # needs tuning
#   lll                 # long lines, not relevant
#   gotype, gotypex     # already done by compiling
#   safesql             # no sql
#   interfacer          # not that useful
lint: check-copyrights
	@echo "Running ${@}"
	@gometalinter \
	--deadline=10m \
	--concurrency=1 \
	--enable-all \
	--enable=golint \
	--enable=errcheck \
	--enable=unconvert \
	--enable=structcheck \
	--enable=misspell \
	--disable=goimports \
	--enable=ineffassign \
	--disable=gofmt \
	--enable=nakedret \
	--enable=megacheck \
	--disable=unparam \
	--disable=gosec \
	--disable=vetshadow \
	--disable=gochecknoinits \
	--disable=gochecknoglobals \
	--disable=dupl \
	--disable=gocyclo \
	--disable=lll \
	--disable=gotype --disable=gotypex \
	--disable=safesql \
	--disable=interfacer \
	--skip=examples \
	--exclude=".*\.pb\.go" \
	--exclude=".*\.dbx\.go" \
	./...

check-copyrights:
	@echo "Running ${@}"
	@./scripts/check-for-header.sh

# Applies goimports to every go file (excluding vendored files)
goimports-fix:
	goimports -w $$(find . -type f -name '*.go' -not -path "*/vendor/*")

proto:
	@echo "Running ${@}"
	./scripts/build-protos.sh

build-dev-deps:
	go get github.com/golang/protobuf/protoc-gen-go
	go get github.com/mattn/goveralls
	go get golang.org/x/tools/cover
	go get github.com/modocache/gover
	go get github.com/alecthomas/gometalinter
	gometalinter --install --force

test: lint
	go install -v ./...
	go test -race -v -covermode=atomic -coverprofile=coverage.out ./...
	gover
	@echo done

test-captplanet:
	@echo "Running ${@}"
	@./scripts/test-captplanet.sh

test-docker:
	docker-compose up -d --remove-orphans test
	docker-compose run test make test

test-docker-clean:
	-docker-compose down --rmi all

images: satellite-image storagenode-image uplink-image
	echo Built version: ${TAG}

.PHONY: satellite-image
satellite-image:
	${DOCKER_BUILD} -t storjlabs/satellite:${TAG}${CUSTOMTAG} -f cmd/satellite/Dockerfile .
.PHONY: storagenode-image
storagenode-image:
	${DOCKER_BUILD} -t storjlabs/storagenode:${TAG}${CUSTOMTAG} -f cmd/storagenode/Dockerfile .
.PHONY: uplink-image
uplink-image:
	${DOCKER_BUILD} -t storjlabs/uplink:${TAG}${CUSTOMTAG} -f cmd/uplink/Dockerfile .

.PHONY: all-in-one
all-in-one:
	if [ -z "${VERSION}" ]; then \
		$(MAKE) images -j 3 \
		&& export VERSION="${TAG}"; \
	fi \
	&& docker-compose up -d storagenode \
	&& scripts/fix-mock-overlay \
	&& docker-compose up storagenode satellite uplink

push-images:
	docker tag storjlabs/satellite:${TAG} storjlabs/satellite:latest
	docker push storjlabs/satellite:${TAG}
	docker push storjlabs/satellite:latest
	docker tag storjlabs/storagenode:${TAG} storjlabs/storagenode:latest
	docker push storjlabs/storagenode:${TAG}
	docker push storjlabs/storagenode:latest
	docker tag storjlabs/uplink:${TAG} storjlabs/uplink:latest
	docker push storjlabs/uplink:${TAG}
	docker push storjlabs/uplink:latest

ifeq (${BRANCH},master)
clean-images:
	-docker rmi storjlabs/satellite:${TAG} storjlabs/satellite:latest
	-docker rmi storjlabs/storagenode:${TAG} storjlabs/storagenode:latest
	-docker rmi storjlabs/uplink:${TAG} storjlabs/uplink:latest
else
clean-images:
	-docker rmi storjlabs/satellite:${TAG}
	-docker rmi storjlabs/storagenode:${TAG}
	-docker rmi storjlabs/uplink:${TAG}
endif

install-deps:
	go get -u -v golang.org/x/vgo
	cd vgo install ./...

.PHONY: deploy
deploy:
	./scripts/deploy.staging.sh satellite storjlabs/satellite:${TAG}
	for i in $(shell seq 1 60); do \
		./scripts/deploy.staging.sh storagenode-$$i storjlabs/storagenode:${TAG}; \
	done

.PHONY: binary
binary: CUSTOMTAG = -${GOOS}-${GOARCH}
binary:
	mkdir -p release/${TAG}
	CUSTOMTAG=$(CUSTOMTAG) $(MAKE) $(COMPONENT)-image
	cid=$$(docker create storjlabs/$(COMPONENT):${TAG}${CUSTOMTAG}) \
	&& docker cp $$cid:/app/$(COMPONENT) release/${TAG}/$(COMPONENT)_${GOOS}_${GOARCH}${FILEEXT} \
    && docker rm $$cid
	docker rmi storjlabs/$(COMPONENT):${TAG}${CUSTOMTAG}
	rm -f release/${TAG}/${COMPONENT}_${GOOS}_${GOARCH}.zip
	cd release/${TAG}; zip ${COMPONENT}_${GOOS}_${GOARCH}.zip ${COMPONENT}_${GOOS}_${GOARCH}${FILEEXT}
	rm -f release/${TAG}/${COMPONENT}_${GOOS}_${GOARCH}${FILEEXT}

# To update this section, modify and run the following:
# for c in satellite storagenode uplink; do \
# for oa in "darwin amd64" "linux 386" \
# "linux amd64" "windows 386" "windows amd64"; do \
# echo "$c $oa"; done; done | while read -r c o a; do; \
# printf ".PHONY: ${c}_${o}_${a}\n${c}_${o}_${a}:\n\tGOOS=${o} GOARCH=${a} COMPONENT=${c} \$(MAKE) binary\n"; \
# done
.PHONY: satellite_darwin_amd64
satellite_darwin_amd64:
	GOOS=darwin GOARCH=amd64 COMPONENT=satellite $(MAKE) binary
.PHONY: satellite_linux_386
satellite_linux_386:
	GOOS=linux GOARCH=386 COMPONENT=satellite $(MAKE) binary
.PHONY: satellite_linux_amd64
satellite_linux_amd64:
	GOOS=linux GOARCH=amd64 COMPONENT=satellite $(MAKE) binary
.PHONY: satellite_windows_386
satellite_windows_386:
	GOOS=windows GOARCH=386 COMPONENT=satellite $(MAKE) binary
.PHONY: satellite_windows_amd64
satellite_windows_amd64:
	GOOS=windows GOARCH=amd64 COMPONENT=satellite $(MAKE) binary
.PHONY: storagenode_darwin_amd64
storagenode_darwin_amd64:
	GOOS=darwin GOARCH=amd64 COMPONENT=storagenode $(MAKE) binary
.PHONY: storagenode_linux_386
storagenode_linux_386:
	GOOS=linux GOARCH=386 COMPONENT=storagenode $(MAKE) binary
.PHONY: storagenode_linux_amd64
storagenode_linux_amd64:
	GOOS=linux GOARCH=amd64 COMPONENT=storagenode $(MAKE) binary
.PHONY: storagenode_windows_386
storagenode_windows_386:
	GOOS=windows GOARCH=386 COMPONENT=storagenode $(MAKE) binary
.PHONY: storagenode_windows_amd64
storagenode_windows_amd64:
	GOOS=windows GOARCH=amd64 COMPONENT=storagenode $(MAKE) binary
.PHONY: uplink_darwin_amd64
uplink_darwin_amd64:
	GOOS=darwin GOARCH=amd64 COMPONENT=uplink $(MAKE) binary
.PHONY: uplink_linux_386
uplink_linux_386:
	GOOS=linux GOARCH=386 COMPONENT=uplink $(MAKE) binary
.PHONY: uplink_linux_amd64
uplink_linux_amd64:
	GOOS=linux GOARCH=amd64 COMPONENT=uplink $(MAKE) binary
.PHONY: uplink_windows_386
uplink_windows_386:
	GOOS=windows GOARCH=386 COMPONENT=uplink $(MAKE) binary
.PHONY: uplink_windows_amd64
uplink_windows_amd64:
	GOOS=windows GOARCH=amd64 COMPONENT=uplink $(MAKE) binary

# To update this section, modify and run the following:
# grep -Eo '^[a-z]*_[a-z]*_[a-z0-9]*' Makefile | tr '\n' ' '
.PHONY: binaries
binaries: satellite_darwin_amd64 satellite_linux_386 satellite_linux_amd64 satellite_windows_386 satellite_windows_amd64 storagenode_darwin_amd64 storagenode_linux_386 storagenode_linux_amd64 storagenode_windows_386 storagenode_windows_amd64 uplink_darwin_amd64 uplink_linux_386 uplink_linux_amd64 uplink_windows_386 uplink_windows_amd64

.PHONY: binaries-upload
binaries-upload:
	cd release; gsutil -m cp -r . gs://storj-v3-alpha-builds

.PHONY: binaries-clean
binaries-clean:
	rm -rf release

clean: test-docker-clean binaries-clean clean-images
