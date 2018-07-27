.PHONY: test lint proto check-copyrights build-dev-deps

TAG    	:= $$(git rev-parse --short HEAD)
GO_VERSION := 1.10
COMPOSE_PROJECT_NAME := ${TAG}-$(shell git rev-parse --abbrev-ref HEAD)
GO_DIRS := $(shell go list ./... | grep -v storj.io/storj/examples)


lint: check-copyrights
	@echo "Running ${@}"
	@gometalinter \
	--deadline=170s \
	--disable-all \
	--enable=golint \
	--enable=errcheck \
	--enable=goimports \
	--enable=vet \
	--enable=deadcode \
	--enable=goconst \
	--exclude=".*\.pb\.go" \
	--exclude=".*_test.go" \
	--exclude="examples/*" \
  ${GO_DIRS}

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
	go get golang.org/x/tools/cmd/cover
	go get github.com/modocache/gover
	go get github.com/alecthomas/gometalinter
	gometalinter --install --force

test: lint
	go install -v ./...
	go test -v -covermode=count -coverprofile=coverage.out ./...
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

images:
	docker build --build-arg VERSION=${GO_VERSION} -t storjlabs/hc:${TAG}-${GO_VERSION} -f cmd/hc/Dockerfile .
	docker tag storjlabs/hc:${TAG}-${GO_VERSION} storjlabs/hc:latest
	docker build -t storjlabs/piecestore-farmer:${TAG} -f cmd/piecestore-farmer/Dockerfile .
	docker tag storjlabs/piecestore-farmer:${TAG} storjlabs/piecestore-farmer:latest

push-images:
	docker push storjlabs/hc:${TAG}-${GO_VERSION}
	docker push storjlabs/hc:latest
	docker push storjlabs/piecestore-farmer:${TAG}
	docker push storjlabs/piecestore-farmer:latest

clean-images:
	-docker rmi storjlabs/hc:${TAG}-${GO_VERSION} storjlabs/hc:latest
	-docker rmi storjlabs/piecestore-farmer:${TAG} storjlabs/piecestore-farmer:latest

install-deps:
	go get -u -v golang.org/x/vgo
	cd vgo install ./...
