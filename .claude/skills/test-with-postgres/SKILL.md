---
name: test-with-postgres
description: Run unit tests that require PostgreSQL. Use this skill when the user wants to run tests with PostgreSQL database backend. Automatically handles checking for and configuring a PostgreSQL Docker container.
allowed-tools: Bash(docker *), Bash(go *), Bash(do *)
---

# Run Unit Tests with PostgreSQL

You are helping run unit tests that require PostgreSQL.

## Instructions

### Starting PostgreSQL

PostgreSQL must be running before tests can execute. Use Docker to start a PostgreSQL instance:

```bash
# Check if postgres container is already running
docker ps --filter name=storj_unit_tests_postgres --format '{{.Names}}'

# If not running, start it:
docker run --rm -d -p 5433:5432 --name storj_unit_tests_postgres -e POSTGRES_PASSWORD=tmppass postgres:17

# Wait for postgres to be ready (retry until successful)
until docker exec storj_unit_tests_postgres psql -h localhost -U postgres -d postgres -c "select 1" > /dev/null 2>&1; do
  echo "Waiting for postgres server..."
  sleep 1
done

# Create test database
docker exec storj_unit_tests_postgres psql -h localhost -U postgres -c "create database teststorj;"
```

Alternatively, run `testsuite/postgres-dev.sh` which handles all setup automatically.

### Running Tests

1. **If test name is provided in arguments**:
   - Find the package containing the test using Grep
   - Run the test with PostgreSQL configured

2. **If only package path is provided**:
   - Run all tests in that package with PostgreSQL configured

3. **Command format**:
```bash
go test -v ./package/path -run TestName -postgres-test-db 'postgres://postgres:tmppass@localhost:5433/teststorj?sslmode=disable'
```

4. **Report test results**:
   - Show whether tests passed or failed
   - List all subtests that ran
   - If tests failed, offer to help investigate the failures

5. **Stop the Docker container after tests complete**:
   ```bash
   docker rm -f storj_unit_tests_postgres
   ```

## Common test paths

Some common test paths in the Storj codebase:
- `./satellite/metabase` - Metabase tests
- `./satellite/metainfo` - Metainfo API tests
- `./satellite/satellitedb` - Database tests

## Example Usage

```bash
# Run a specific test
go test -v ./satellite/metainfo -run TestEndpoint_Object_No_StorageNodes -postgres-test-db 'postgres://postgres:tmppass@localhost:5433/teststorj?sslmode=disable'

# Run all tests in a package
go test -v ./satellite/metabase -postgres-test-db 'postgres://postgres:tmppass@localhost:5433/teststorj?sslmode=disable'

# Run tests with timeout
go test -v -timeout 10m ./satellite/metabase -run TestLoop -postgres-test-db 'postgres://postgres:tmppass@localhost:5433/teststorj?sslmode=disable'
```

## Cleanup

To stop the PostgreSQL container when done:
```bash
docker rm -f storj_unit_tests_postgres
```

## Notes

- The container uses port 5433 to avoid conflicts with any local PostgreSQL installation on the default port 5432
- Tests automatically create isolated test databases for each test run
- Use `STORJ_TEST_POSTGRES='omit'` to skip PostgreSQL tests
