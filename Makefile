.PHONY: test lint proto check-copyrights build-dev-deps release release-osx release-windows release-linux


GO_VERSION ?= 1.10
COMPOSE_PROJECT_NAME := ${TAG}-$(shell git rev-parse --abbrev-ref HEAD)
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
ifeq (${BRANCH},master)
TAG    	:= $(shell git rev-parse --short HEAD)-go${GO_VERSION}
else
TAG    	:= $(shell git rev-parse --short HEAD)-${BRANCH}-go${GO_VERSION}
endif


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

build-binaries:
	docker build -t hc .

run-hc:
	docker network create test-net

	docker run -d \
		--name redis \
		--network test-net \
		-p 127.0.0.1:6379:6379 \
		redis

	docker run -d \
		--name=hc \
		--network test-net \
		-p 127.0.0.1:8080:8080 \
		-e REDIS_ADDRESS=redis:6379 \
		-e REDIS_PASSWORD="" \
		-e REDIS_DB=1 \
		-e OVERLAY_PORT=7070 \
		hc

test-captplanet:
	@echo "Running ${@}"
	@./scripts/test-captplanet.sh

clean-local:
	# cleanup heavy client
	docker stop hc || true
	docker rm hc || true
	# cleanup redis
	docker stop redis || true
	docker rm redis || true
	# cleanup docker network
	docker network rm test-net || true

test-docker:
	docker-compose up -d --remove-orphans test
	docker-compose run test make test

test-docker-clean:
	-docker-compose down --rmi all

images: satellite-image storage-node-image uplink-image
	echo Built version: ${TAG}

.PHONY: satellite-image
satellite-image:
	docker build --build-arg GO_VERSION=${GO_VERSION} -t storjlabs/satellite:${TAG} -f cmd/hc/Dockerfile .
.PHONY: storage-node-image
storage-node-image:
	docker build --build-arg GO_VERSION=${GO_VERSION} -t storjlabs/storage-node:${TAG} -f cmd/storagenode/Dockerfile .
.PHONY: uplink-image
uplink-image:
	docker build --build-arg GO_VERSION=${GO_VERSION} -t storjlabs/uplink:${TAG} -f cmd/uplink/Dockerfile .

.PHONY: all-in-one
all-in-one:
	if [ -z "${VERSION}" ]; then \
		$(MAKE) images -j 3 \
		&& export VERSION="${TAG}"; \
	fi \
	&& docker-compose up -d storage-node \
	&& scripts/fix-mock-overlay \
	&& docker-compose up storage-node satellite uplink

push-images:
	docker tag storjlabs/satellite:${TAG} storjlabs/satellite:latest
	docker push storjlabs/satellite:${TAG}
	docker push storjlabs/satellite:latest
	docker tag storjlabs/storage-node:${TAG} storjlabs/storage-node:latest
	docker push storjlabs/storage-node:${TAG}
	docker push storjlabs/storage-node:latest
	docker tag storjlabs/uplink:${TAG} storjlabs/uplink:latest
	docker push storjlabs/uplink:${TAG}
	docker push storjlabs/uplink:latest

ifeq (${BRANCH},master)
clean-images:
	-docker rmi storjlabs/satellite:${TAG} storjlabs/satellite:latest
	-docker rmi storjlabs/storage-node:${TAG} storjlabs/storage-node:latest
	-docker rmi storjlabs/uplink:${TAG} storjlabs/uplink:latest
else
clean-images:
	-docker rmi storjlabs/satellite:${TAG}
	-docker rmi storjlabs/storage-node:${TAG}
	-docker rmi storjlabs/uplink:${TAG}
endif

install-deps:
	go get -u -v golang.org/x/vgo
	cd vgo install ./...

.PHONY: deploy
deploy:
	./scripts/deploy.staging.sh satellite storjlabs/satellite:${TAG}
	for i in $(shell seq 1 60); do \
		./scripts/deploy.staging.sh storage-node-$$i storjlabs/storage-node:${TAG}; \
	done

release-osx:
	GOOS=darwin GOARCH=amd64 go build -o release/uplink-osx-amd64/uplink ./cmd/uplink
	cd release; tar czvf uplink-osx-amd64.tar.gz uplink-osx-amd64
	rm -rf release/uplink-osx-amd64

release-linux:
	GOOS=linux GOARCH=amd64 go build -o release/uplink-linux-amd64/uplink ./cmd/uplink
	cd release; tar czvf uplink-linux-amd64.tar.gz uplink-linux-amd64
	rm -rf release/uplink-linux-amd64

release-windows:
	GOOS=windows GOARCH=amd64 go build -o release/uplink-windows-amd64/uplink ./cmd/uplink
	cd release; zip uplink-windows-amd64.zip uplink-windows-amd64
	rm -rf release/uplink-windows-amd64

release: release-osx release-linux release-windows
