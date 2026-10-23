---
name: test-with-tidb
description: Run Storj unit tests against TiDB (with the PostgreSQL master database), e.g. metabase on TiDB, the TiDB bucket eventing outbox, tidbutil.
allowed-tools: Bash(make test/setup/tidb), Bash(docker compose *), Bash(go *)
---

# Run Unit Tests with TiDB

TiDB tests need two databases: TiDB for the metabase, and PostgreSQL as the "master" (satellite) database,
because satellitedb does not run on TiDB yet. The TiDB connection string carries the Postgres one after `!!master=`.

## Start the databases (once)

From the repository root:

```bash
# Already up? Then skip the setup — it starts with `down -v` and would wipe the running stack.
docker compose -f testsuite/docker-compose.tests.yaml ps --status running postgres tidb

make test/setup/tidb
```

`make test/setup/tidb` starts `postgres` (port 5532) and `tidb` (port 4400) from `testsuite/docker-compose.tests.yaml`,
creates `teststorj`, and waits until TiDB accepts connections. The compose TiDB mounts `testsuite/tidb.toml`, whose
`max-index-length = 12288` the metabase needs (a plain `pingcap/tidb` container fails the migration with
`Error 1071: Specified key was too long ... 3072 bytes`).

Keep the containers running between test runs; TiDB start-up is slow.

## Run tests

```bash
export STORJ_TEST_TIDB='tidb://root@localhost:4400/storj?parseTime=true!!master=postgres://postgres:postgres@localhost:5532/teststorj?sslmode=disable'
export STORJ_TEST_POSTGRES=omit STORJ_TEST_COCKROACH=omit

go test -vet=off ./satellite/eventing -run TestTiDBEventSource
```

- `STORJ_TEST_POSTGRES=omit STORJ_TEST_COCKROACH=omit` is required: without it the Postgres/Cockroach variants of
  `metabasetest.Run*` tests **fail** (not skip) with "connection string not provided", because
  `satellite/metabase/metabasetest/run.go` does not skip on an empty URL like `satellitedbtest.Run` does.
- Add `-v` only together with `-run`; on a whole package it prints a lot. Add `-race` once before submitting
  (CI runs with it), not on every iteration.
- Long packages such as `./satellite/metabase` need `-timeout 20m`.
- If the test name is given without a package, find the package with Grep first.

Report pass/fail and the subtests that ran. Check that the `TiDB` subtests really ran: a test without a TiDB
variant still reports `ok` with only the omitted variants skipped.

Common TiDB test packages: `./satellite/metabase`, `./satellite/eventing` (`TestTiDBEventSource`,
`TestTiDBEventingEndToEnd`), `./shared/dbutil/tidbutil`.

## Cleanup

When the user is done:
```bash
docker compose -f testsuite/docker-compose.tests.yaml down -v
```
