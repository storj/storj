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
	--enable=gosimple \
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
	go get -u github.com/golang/protobuf/protoc-gen-go
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install --force

test:
	go test -v ./...

install-deps:
	git clone https://github.com/storj/storj-vendor.git
	mv storj-vendor/src ./vendor
	go get -t -v ./... || true
	ln -s $(GOPATH)/src/storj.io/storj $(GOPATH)/src/github.com/storj/storj