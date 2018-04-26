.PHONY: test lint

lint: check-copyrights
	@echo "Running ${@}"
	@gometalinter \
	--deadline=60s \
	--disable-all \
	--enable=golint \
	--enable=goimports \
	--enable=vet \
	--enable=deadcode \
	--enable=goconst \
	--exclude=.*\.pb\.go \
	--exclude=.*_test.go \
	./...

check-copyrights:
	@echo "Running ${@}"
	@./scripts/check-for-header.sh


proto:
	@echo "Running ${@}"
	./scripts/build-protos.sh


build-dev-deps:
	@echo "PATH is: ${PATH}"
	go get -u github.com/golang/protobuf/protoc-gen-go
	go get -u github.com/alecthomas/gometalinter
	go get -u github.com/spf13/viper
	go get -u github.com/tyler-smith/go-bip39
	go get -u github.com/zeebo/errs
	go get -u github.com/vivint/infectious
	go get -u golang.org/x/crypto/nacl/secretbox
	go get -u google.golang.org/grpc
	go get -u github.com/go-redis/redis
	go get -u github.com/gogo/protobuf/proto
	gometalinter --install --force

test:
	go test -v ./...
