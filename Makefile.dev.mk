##@ Development

# backwards compatible alias for bump
.PHONY: bump-dependencies
bump-dependencies: bump

.PHONY: bump
bump: ## Bump common and uplink dependencies in all modules
	go get storj.io/common@main storj.io/uplink@main
	go mod tidy
	cd testsuite/playwright-ui;\
		go get storj.io/common@main storj.io/uplink@main;\
		go mod tidy;
	cd testsuite/storjscan;\
		go get storj.io/common@main storj.io/uplink@main;\
		go mod tidy;

.PHONY: protolock
protolock: ## Update protolock state
	protolock commit --ignore "satellite/internalpb,storagenode/internalpb"

.PHONY: goimports-fix
goimports-fix: ## Applies goimports to every go file (excluding vendored files)
	goimports -w -local storj.io $$(find . -type f -name '*.go' -not -path "*/vendor/*")

.PHONY: goimports-st
goimports-st: ## Applies goimports to every go file in `git status` (ignores untracked files)
	@git status --porcelain -uno|grep .go|grep -v "^D"|sed -E 's,\w+\s+(.+->\s+)?,,g'|xargs -I {} goimports -w -local storj.io {}

##@ Lint

LINT_TARGET="./..."

.PHONY: llint
llint: ## Run all linting tools using local tools
	go run ./scripts/lint.go \
		-parallel 8 \
		-race \
		-modules \
		-copyright \
		-imports \
		-peer-constraints \
		-atomic-align \
		-monkit \
		-errs \
		-staticcheck \
		-golangci \
		-wasm-size \
		-protolock \
		-check-tx \
		$(LINT_TARGET)

.PHONY: lint
lint: ## Run all linting tools with our CI image
	docker run --rm -it \
		-v ${GOPATH}/pkg:/go/pkg \
		-v ${PWD}:/storj \
		-w /storj \
		storjlabs/ci:slim \
		make llint LINT_TARGET="$(LINT_TARGET)"

.PHONY: bump-lint
bump-lint: ## Bump all linting tools
	curl https://raw.githubusercontent.com/storj/ci/refs/heads/main/images/tools | xargs -n1 go get -modfile ./scripts/go.mod -tool
	curl https://raw.githubusercontent.com/storj/ci/refs/heads/main/images/internal-tools | xargs -n1 go get -modfile ./scripts/go.mod -tool
	curl https://raw.githubusercontent.com/storj/ci/refs/heads/main/.golangci.yml -o .golangci.yml	
	cd scripts;\
		go mod tidy;