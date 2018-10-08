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
	--build-arg GO_VERSION=${GO_VERSION}

lint: check-copyrights
	@echo "Running ${@}"
	@golangci-lint run

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
	go get github.com/mattn/goveralls
	go get golang.org/x/tools/cover
	go get github.com/modocache/gover
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b ${GOPATH}/bin v1.10.2

test:
	go test -race -v -cover -coverprofile=.coverprofile ./...
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
	for deployment in $$(kubectl --context nonprod -n v3 get deployment -l app=storagenode --output=jsonpath='{.items..metadata.name}'); do \
		kubectl --context nonprod --namespace v3 patch deployment $$deployment -p"{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"storagenode\",\"image\":\"storjlabs/storagenode:${TAG}\"}]}}}}" ; \
	done
	kubectl --context nonprod --namespace v3 patch deployment satellite -p"{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"satellite\",\"image\":\"storjlabs/satellite:${TAG}\"}]}}}}"

.PHONY: binary
binary: CUSTOMTAG = -${GOOS}-${GOARCH}
binary:
	@if [ -z "${COMPONENT}" ]; then echo "Try one of the following targets instead:" \
		&& for b in binaries ${BINARIES}; do echo "- $$b"; done && exit 1; fi
	mkdir -p release/${TAG}
	tar -c . | docker run --rm -i -e TAR=1 -e GO111MODULE=on \
	-e GOOS=${GOOS} -e GOARCH=${GOARCH} -e CGO_ENABLED=1 \
	-w /go/src/storj.io/storj storjlabs/golang \
	-o app storj.io/storj/cmd/${COMPONENT} \
	| tar -O -x ./app > release/${TAG}/$(COMPONENT)_${GOOS}_${GOARCH}${FILEEXT}
	chmod 755 release/${TAG}/$(COMPONENT)_${GOOS}_${GOARCH}${FILEEXT}
	rm -f release/${TAG}/${COMPONENT}_${GOOS}_${GOARCH}.zip
	cd release/${TAG}; zip ${COMPONENT}_${GOOS}_${GOARCH}.zip ${COMPONENT}_${GOOS}_${GOARCH}${FILEEXT}
	rm -f release/${TAG}/${COMPONENT}_${GOOS}_${GOARCH}${FILEEXT}

.PHONY: satellite_%
satellite_%:
	GOOS=$(word 2, $(subst _, ,$@)) GOARCH=$(word 3, $(subst _, ,$@)) COMPONENT=satellite $(MAKE) binary
.PHONY: storagenode_%
storagenode_%:
	GOOS=$(word 2, $(subst _, ,$@)) GOARCH=$(word 3, $(subst _, ,$@)) COMPONENT=storagenode $(MAKE) binary
.PHONY: uplink_%
uplink_%:
	GOOS=$(word 2, $(subst _, ,$@)) GOARCH=$(word 3, $(subst _, ,$@)) COMPONENT=uplink $(MAKE) binary

COMPONENTLIST := uplink satellite storagenode
OSARCHLIST    := linux_amd64 windows_amd64 darwin_amd64
BINARIES      := $(foreach C,$(COMPONENTLIST),$(foreach O,$(OSARCHLIST),$C_$O))
.PHONY: binaries
binaries: ${BINARIES}

.PHONY: binaries-upload
binaries-upload:
	cd release; gsutil -m cp -r . gs://storj-v3-alpha-builds

.PHONY: binaries-clean
binaries-clean:
	rm -rf release

clean: test-docker-clean binaries-clean clean-images
