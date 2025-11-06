---
name: test-with-spanner
description: Run unit tests that require the Spanner emulator. Use this skill when the user wants to run tests in packages like satellite/metabase, satellite/metainfo, or any other tests that interact with Spanner. Automatically handles checking for and configuring the Spanner emulator environment.
allowed_tools: echo, go, spanner_emulator
---

# Run Unit Tests with Spanner

You are helping run unit tests that require the Spanner emulator.

## Instructions

Follow these steps to run unit tests with Spanner:

1. **Check if the Spanner emulator environment is already configured**:
   - Check if `SPANNER_EMULATOR_HOST` and `STORJ_TEST_SPANNER` environment variables are set
   - Use bash to check: `echo $SPANNER_EMULATOR_HOST` and `echo $STORJ_TEST_SPANNER`

2. **If environment variables are already set**:
   - Inform the user that the Spanner emulator environment is already configured
   - Show the current values of the environment variables
   - Ask the user which test they want to run (package path and optional test name)
   - Run the test directly using: `go test -v ./package/path -run TestName`

3. **If environment variables are NOT set**:
   - Inform the user that the Spanner emulator needs to be started
   - Start the Spanner emulator in the background: `spanner_emulator --host_port 127.0.0.1:10008`
   - Save the emulator process ID for cleanup later
   - Ask the user which test they want to run (package path and optional test name)
   - Run the test with environment variables:
     ```bash
     SPANNER_EMULATOR_HOST=localhost:10008 \
     STORJ_TEST_SPANNER=spanner://127.0.0.1:10008?emulator \
     go test -v ./package/path -run TestName
     ```
   - After the test completes, ask the user if they want to stop the emulator or keep it running for more tests
   - If they want to stop it, kill the emulator process

4. **Report test results**:
   - Show the test output
   - Indicate whether tests passed or failed
   - If tests failed, offer to help investigate the failures

## Common test paths

Some common test paths in the Storj codebase:
- `./satellite/metabase` - Metabase tests
- `./satellite/metainfo` - Metainfo API tests
- `./satellite/satellitedb` - Database tests

## Notes

- The standard Spanner emulator port is `127.0.0.1:10007` but flag `--host_port 127.0.0.1:10008` can be used to change it.
- If the emulator is started, it will continue running until explicitly stopped
- Multiple tests can be run with the same emulator instance
- The emulator process runs in the background and should be cleaned up when done
