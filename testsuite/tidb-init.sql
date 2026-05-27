-- Bootstrap SQL run by TiDB once on initial cluster init, via the
-- bootstrap-sql-file option in tidb.toml.

CREATE DATABASE IF NOT EXISTS storj;

-- async_commit lowers commit latency by acking before the secondary lock
-- cleanup completes. tidb_enable_1pc is intentionally NOT enabled here:
-- on v8.5.6 it triggers MVCC "assertion: NotExist failed" under our
-- concurrent commit patterns. async_commit alone is safe.
SET GLOBAL tidb_enable_async_commit = 1;

-- Skip UTF-8 validation on writes — saves CPU on every insert; tests
-- don't exercise invalid encodings.
SET GLOBAL tidb_skip_utf8_check = 1;

-- LOCAL runtime filters apply within a single TiDB instance only, which
-- is what we have in this single-node test setup.
SET GLOBAL tidb_runtime_filter_mode = 'LOCAL';
