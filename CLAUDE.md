# Storj Development Guide for Claude

## Build/Test/Lint Commands

- Build: `go build ./...` or `go build ./cmd/[component]`
- Test All: `make test`
- Single Test: `go test -v ./package/path -run TestName`
- Lint: `make llint`
- Lint only one package: `make llint LINT_TARGET=./<directory>`

## Code Style Guidelines

- Packages: Use `storj.io/storj/...` import paths
- Commit style: `{scope}: {message}` format (e.g., `satellite/metainfo: add validation`). Never add AI attribution (author, committer or other additional lines) to the commit.
- Error handling: Always check errors, use proper context. Use `github.com/zeebo/errs`. Wrap errors from external sources.
- Naming: Use descriptive names, prefer clarity over brevity
- Go formatting: Follow standard Go conventions (`gofmt`)
- Import order and groupping:
    * First group should include standard golang SDK libraries
    * Next group the 3rd party libraries
    * Last group: all the `storj.io` libraries.
- Import orders can be forced with `gci write --section Standard --section Default --section 'Prefix(storj.io/)'`
- Use `monkit` instrumentation patterns for metrics (Usually it's the `defer mon.Task()(&ctx)(&err)` pattern)
- Comments: Use meaningful comments for complex code sections. Only mention the return types when it add additional info.
- Tests: Write comprehensive tests for new functionality

## Repository Structure

Storj is a distributed cloud storage network with components:
- `./satellite`: Manages metadata, payments, and network orchestration
- `./storagenode`: Stores encrypted file pieces
- `./multinode`: Component to manage multiple storagenodes together
- `./shared` and `./private`: generic tools which can be used from both, or even from external locations (prefer `./shared`)

