lint: check-copyrights
	@echo "Running ${@}"
	@gometalinter.v2 \
	--deadline=60s \
	--disable-all \
	--enable=golint \
	--enable=goimports \
	--enable=vet \
	--enable=deadcode \
	--enable=gosimple \
	--exclude=.*\.pb\.go \
	./...

check-copyrights:
	@echo "Running ${@}"
	@./scripts/check-for-header.sh


proto:
	@echo "Running ${@}"
	./scripts/build-protos.sh


build-dev-deps:
	go get -u github.com/golang/protobuf/protoc-gen-go
	go get -u gopkg.in/alecthomas/gometalinter.v2
	gometalinter.v2 --install
