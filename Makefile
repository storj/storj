lint:
	@echo "gometalinter"
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


proto:
	@echo "Running ${@}"
	./scripts/build-protos.sh