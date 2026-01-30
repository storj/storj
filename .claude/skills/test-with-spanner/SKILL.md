---
name: test-with-spanner
description: Run unit tests that require the Spanner emulator. Use this skill when the user wants to run tests in packages like satellite/metabase, satellite/metainfo, or any other tests that interact with Spanner. Automatically handles checking for and configuring the Spanner emulator environment.
allowed-tools: Bash(go *)
---

# Run Unit Tests with Spanner

You are helping run unit tests that require the Spanner emulator.

## Instructions

The Storj test framework automatically manages the Spanner emulator lifecycle using the `run:` prefix in the `STORJ_TEST_SPANNER` environment variable.
To run tests automatically `spanner_emulator` binary needs to be on PATH. Spanner can be also set for tests using `-spanner-test-db` flag.

### Running Tests

1. **If test name is provided in arguments**:
   - Find the package containing the test using Grep
   - Run the test with the auto-managed emulator

2. **If only package path is provided**:
   - Run all tests in that package with the auto-managed emulator

3. **Command format**:
```bash
go test -v ./package/path -run TestName -spanner-test-db 'run:spanner_emulator'
```

   The `run:` prefix tells the test framework to:
   - Automatically start the Spanner emulator before tests
   - Configure the connection for each test
   - Clean up and stop the emulator after tests complete

4. **Report test results**:
   - Show whether tests passed or failed
   - List all subtests that ran
   - If tests failed, offer to help investigate the failures

## Common test paths

Some common test paths in the Storj codebase:
- `./satellite/metabase` - Metabase tests
- `./satellite/metainfo` - Metainfo API tests
- `./satellite/satellitedb` - Database tests

## Example Usage

```bash
# Run a specific test
go test -v ./satellite/metainfo -run TestEndpoint_Object_No_StorageNodes -spanner-test-db 'run:spanner_emulator'

# Run all tests in a package
go test -v ./satellite/metabase -spanner-test-db 'run:spanner_emulator'

# Run tests with timeout
go test -v -timeout 10m ./satellite/metabase -run TestLoop -spanner-test-db 'run:spanner_emulator'
```

## Notes

- Each test gets its own emulator instance that's automatically managed
- No manual cleanup is required - the framework handles emulator lifecycle
- The `run:` prefix is the recommended approach used in Storj's CI/CD (see Jenkinsfile.verify and Jenkinsfile.public)
- Alternative: If `STORJ_TEST_SPANNER` is already set to a running emulator, tests will use that instead
