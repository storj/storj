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
	