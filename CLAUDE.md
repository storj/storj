# Storj Development Guide for Claude

## Overview

Storj is a decentralized cloud storage network where data is encrypted, split into pieces using erasure coding, and distributed across thousands of storage nodes worldwide. This repository contains the core components that power the network.

### Key Architectural Concepts

**Data Flow**: Files → Encryption → Erasure Coding (80 pieces, 29 needed to recover) → Distributed Storage Nodes → Metadata stored in Satellite

**Core Components**:
- **Satellite**: Centralized coordinator managing metadata, node selection, payments, and network health
- **Storage Nodes**: Distributed edge nodes storing encrypted file pieces
- **Uplink**: Client library for uploading/downloading data (separate repository)

## Build/Test/Lint Commands

- Build: `go build ./...` or `go build ./cmd/[component]`
- Test All: `make test`
- Single Test: `go test -v ./package/path -run TestName`
- Lint: `make llint`
- Lint only one package: `make llint LINT_TARGET=./<directory>`
- Format imports: `gci write --section Standard --section Default --section 'Prefix(storj.io/)' <file>`

### Running Unit Tests with Spanner

Unit tests require the Spanner emulator to be running. First, check if your environment is already configured:

```bash
# Check if Spanner emulator environment is already set up
if [ -n "$SPANNER_EMULATOR_HOST" ] && [ -n "$STORJ_TEST_SPANNER" ]; then
    echo "Spanner emulator environment already configured"
    echo "SPANNER_EMULATOR_HOST: $SPANNER_EMULATOR_HOST"
    echo "STORJ_TEST_SPANNER: $STORJ_TEST_SPANNER"
    # You can run tests directly
    go test -v ./satellite/metabase -run TestCommitObject
else
    echo "Spanner emulator environment not configured"
    # Follow the setup steps below
fi
```

If the environment variables are **not** already set, follow this workflow:

1. Start the Spanner emulator: `spanner_emulator --host_port 127.0.0.1:10008`
2. Set required environment variables:
   - `SPANNER_EMULATOR_HOST=localhost:10008`
   - `STORJ_TEST_SPANNER=spanner://127.0.0.1:10008?emulator`
3. Run tests: `go test -v ./package/path -run TestName`
4. Stop the emulator when done

Example workflow (when environment is not configured):
```bash
# Start emulator in background
spanner_emulator --host_port 127.0.0.1:10008 &
EMULATOR_PID=$!

# Run tests with environment variables
SPANNER_EMULATOR_HOST=localhost:10008 \
STORJ_TEST_SPANNER=spanner://127.0.0.1:10008?emulator \
go test -v ./satellite/metabase -run TestCommitObject

# Stop emulator
kill $EMULATOR_PID
```

## Code Style Guidelines

- Packages: Use `storj.io/storj/...` import paths
- Commit style: `{scope}: {message}` format (e.g., `satellite/metainfo: add validation`). Never add AI attribution (author, committer or other additional lines) to the commit.
- Error handling: Always check errors, use proper context. Use `github.com/zeebo/errs`. Wrap errors from external sources.
- Naming: Use descriptive names, prefer clarity over brevity
- Go formatting: Follow standard Go conventions (`gofmt`)
- Import order and grouping:
    * First group should include standard golang SDK libraries
    * Next group the 3rd party libraries
    * Last group: all the `storj.io` libraries.
- Import orders can be forced with `gci write --section Standard --section Default --section 'Prefix(storj.io/)'`
- Use `monkit` instrumentation patterns for metrics (Usually it's the `defer mon.Task()(&ctx)(&err)` pattern)
- Comments: Use meaningful comments for complex code sections. Only mention the return types when it add additional info.
- Tests: Write comprehensive tests for new functionality

## Repository Structure

Storj is a distributed cloud storage network with components:
- `./satellite`: Manages metadata, payments, and network orchestration (42 subsystems)
- `./storagenode`: Stores encrypted file pieces
- `./multinode`: Component to manage multiple storagenodes together
- `./shared` and `./private`: generic tools which can be used from both, or even from external locations (prefer `./shared`)
- `./cmd`: Command-line entry points for all components
- `./web`: Frontend applications (Vue 3 + TypeScript for satellite console and storage node dashboard)

### Satellite Subsystems (./satellite)

The satellite is a modular monolith with 42 subsystems. Key areas:

**Metadata & Storage**:
- `metabase`: Core metadata storage (segments, objects, buckets) - uses PostgreSQL/CockroachDB/Spanner
- `metainfo`: gRPC API for object metadata operations (upload/download/delete)
- `buckets`: Bucket management

**Data Integrity & Repair**:
- `repair/checker`: Scans for under-replicated segments
- `repair/repairer`: Coordinates piece repair with storage nodes
- `repair/queue`: Prioritizes segments needing repair
- `audit`: Verifies storage nodes are storing data correctly

**Node Management**:
- `overlay`: Node discovery and reputation tracking
- `contact`: Storage node check-ins and communication
- `nodeselection`: Algorithms for selecting nodes (geofencing, subnet diversity)
- `nodeevents`: Tracks node lifecycle events
- `reputation`: Manages node reliability scores

**Accounting & Billing**:
- `accounting`: Tracks storage and bandwidth usage
- `accounting/tally`: Counts stored data
- `accounting/rollup`: Aggregates usage for billing
- `payments`: Stripe integration for customer billing
- `compensation`: Storage node payout calculations

**User Management**:
- `console`: Web console backend (user, project, API key management)
- `console/consoleweb`: HTTP API and web server
- `admin`: Administrative API for support operations

**Garbage Collection**:
- `gc/sender`: Sends bloom filters to storage nodes
- `gc/bloomfilter`: Generates filters of retained pieces
- `gc/piecetracker`: Tracks piece deletions

**Other Systems**:
- `orders`: Bandwidth allocation and settlement
- `gracefulexit`: Managed node departure protocol
- `emission`: Carbon emissions tracking
- `analytics`: Usage analytics and telemetry

## Architecture Patterns

### Peer Pattern
The satellite uses a "Peer" architecture where services are composed into a single process:
- `satellite/peer.go`: Defines the main Peer struct and all service dependencies
- `satellite/api.go`: Extends Peer with API endpoints
- Services are initialized in dependency order and registered with the peer

### Database Abstraction (DBX)
- Schema defined in `.dbx` files under `satellite/satellitedb/dbx/`
- DBX generates Go code for PostgreSQL, CockroachDB, and Spanner
- Access via interface methods on `satellite.DB` (see `satellite/peer.go:96-156`)
- Example: `DB.OverlayCache()` returns the overlay database interface

### Ranged Loop Pattern
For efficient processing of large datasets:
- `satellite/metabase/rangedloop`: Parallel processing of metabase segments
- Multiple observers can process segments concurrently
- Used by repair checker, garbage collection, accounting tally

### Monkit Instrumentation
All service methods should include monitoring:
```go
func (s *Service) Method(ctx context.Context) (err error) {
    defer mon.Task()(&ctx)(&err)  // Tracks execution time and errors
    // ... implementation
}
```

### Error Wrapping
Use `github.com/zeebo/errs` for error classes:
```go
var Error = errs.Class("mypackage")
return Error.Wrap(err)  // Wrap external errors
return Error.New("description")  // Create new errors
```

### Configuration
- All services use struct-based configuration
- Configs are composed in `satellite.Config` (see `satellite/peer.go:176-268`)
- Can be set via CLI flags, environment variables, or YAML files

## Common Workflows

### Adding a New Satellite Service

1. Create package under `./satellite/myservice/`
2. Define Config struct and Service struct
3. Add database interface to `satellite.DB` if needed
4. Add DBX schema file if using database
5. Add config to `satellite.Config`
6. Initialize service in `satellite/peer.go` or `satellite/api.go`
7. Wire up dependencies (DB, other services)
8. Add monkit instrumentation
9. Write tests

### Database Schema Changes

1. Modify `.dbx` files in `satellite/satellitedb/dbx/`
2. Run `go generate ./satellite/satellitedb/...` to regenerate code
3. Create migration in `satellite/satellitedb/migrations/`
4. Test with `make test-postgres` and `make test-cockroach`

### Adding API Endpoints

**Console API** (HTTP):
- Add methods to `satellite/console/service.go`
- Add HTTP handlers in `satellite/console/consoleweb/`
- Update generated API docs in `satellite/console/consoleweb/consoleapi/apidocs.gen.md`

**Metainfo API** (gRPC):
- Modify `satellite/metainfo/endpoint.go`
- Update proto definitions if needed

**Admin API**:
- Add endpoints in `satellite/admin/back-office/`

### Running Local Development Environment

See `DEVELOPING.md` for detailed instructions:
- Use `storj-up` for local multi-node setup
- Supports PostgreSQL and CockroachDB backends
- Frontend development requires npm for web applications

## Domain Glossary

- **Stripe**: A fixed-size subdivision of a remote segment that serves as the erasure encoding boundary. Each stripe is erasure encoded individually to generate multiple erasure shares. Stripes are the unit on which audits are performed.
- **Erasure Share**: Output of erasure coding a single stripe. Each stripe generates multiple erasure shares with indices (e.g., 0, 1, 2, ..., n-1). Only a subset of erasure shares are needed to recover the original stripe (e.g., 29 out of 80 shares).
- **Piece**: Data stored on a single storage node for a remote segment. A piece is the concatenation of all erasure shares with the same index from all stripes in that segment. For example, the piece at index 3 contains the 3rd erasure share from every stripe in the segment.
- **Segment**: A portion of a file (typically 64 MB). Remote segments are composed of multiple stripes that are erasure encoded. Inline segments (small data) are stored directly without erasure encoding.
- **Object**: A complete file in the metabase (may consist of multiple segments)
- **Node ID**: Unique identifier for a storage node (derived from node's public key)
- **Satellite ID**: Unique identifier for a satellite
- **Uplink**: Client software for uploading/downloading data
- **Metabase**: The database storing object/segment metadata
- **Order**: Cryptographic authorization for bandwidth usage
- **Graceful Exit**: Protocol for storage nodes to leave network safely
- **Audit**: Process of verifying storage nodes store data correctly
- **Repair**: Process of regenerating lost pieces on new nodes
- **Tally**: Counting stored data for accounting purposes
- **Rollup**: Aggregating usage data over time periods

## Testing

### Test Types
- **Unit tests**: Standard Go tests alongside code (`*_test.go`)
- **Integration tests**: Use testplanet to simulate satellite + storage nodes
- **DBX tests**: Test database operations with multiple backends
- **Backward compatibility tests**: In `testsuite/backward-compatibility/`
- **UI tests**: In `web/satellite/tests/` (Playwright)

### Test Patterns
- Use `testplanet` for integration tests requiring full satellite + nodes
- Mock external dependencies (Stripe, email services)
- Use `satellite.DB.Testing()` for database test utilities
- Prefer table-driven tests for multiple input scenarios

## Additional Resources

- **DEVELOPING.md**: Detailed development setup and workflows
- **CONTRIBUTING.md**: How to contribute code and report issues
- **docs/testplan/**: Test plan templates and existing test plans
- **satellite/console/consoleweb/consoleapi/apidocs.gen.md**: Console API reference
- **satellite/admin/back-office/api-docs.gen.md**: Admin API reference
- Design documents: https://github.com/storj/design-docs

