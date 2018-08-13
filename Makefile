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
	--exclude=".*\.dbx\.go" \
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
	go get golang.org/x/tools/cover
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

test-captplanet:
	captplanet setup
	captplanet run &

	aws configure set aws_access_key_id insecure-dev-access-key
	aws configure set aws_secret_access_key insecure-dev-secret-key
	aws configure set default.region us-east-1
	aws configure set default.s3.multipart_threshold 1TB

	head -c 1024 </dev/urandom > ./small-upload-testfile # create 1mb file of random bytes (inline)
	head -c 5120 </dev/urandom > ./big-upload-testfile # create 5mb file of random bytes (remote)

	aws s3 --endpoint=http://localhost:7777/ cp ./small-upload-testfile s3://bucket/small-testfile
	aws s3 --endpoint=http://localhost:7777/ cp ./big-upload-testfile s3://bucket/big-testfile

	aws s3 --endpoint=http://localhost:7777/ ls s3://bucket

	aws s3 --endpoint=http://localhost:7777/ cp s3://bucket/small-testfile ./small-download-testfile
	aws s3 --endpoint=http://localhost:7777/ cp s3://bucket/big-testfile ./big-download-testfile

	if cmp ./small-upload-testfile ./small-download-testfile; then echo "Downloaded file matches uploaded file"; else echo "Downloaded file does not match uploaded file"; exit 1; fi
	if cmp ./big-upload-testfile ./big-download-testfile; then echo "Downloaded file matches uploaded file"; else echo "Downloaded file does not match uploaded file"; exit 1; fi

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
	docker build --build-arg VERSION=${GO_VERSION} -t storjlabs/satellite:${TAG}-${GO_VERSION} -f cmd/hc/Dockerfile .
	docker tag storjlabs/satellite:${TAG}-${GO_VERSION} storjlabs/satellite:latest
	docker build -t storjlabs/storage-node:${TAG} -f cmd/piecestore-farmer/Dockerfile .
	docker tag storjlabs/storage-node:${TAG} storjlabs/storage-node:latest

push-images:
	docker push storjlabs/satellite:${TAG}-${GO_VERSION}
	docker push storjlabs/satellite:latest
	docker push storjlabs/storage-node:${TAG}
	docker push storjlabs/storage-node:latest

clean-images:
	-docker rmi storjlabs/satellite:${TAG}-${GO_VERSION} storjlabs/satellite:latest
	-docker rmi storjlabs/storage-node:${TAG} storjlabs/storage-node:latest

install-deps:
	go get -u -v golang.org/x/vgo
	cd vgo install ./...
