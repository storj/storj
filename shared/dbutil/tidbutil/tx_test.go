// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/dx"
	"storj.io/storj/shared/dbutil/tidbutil"
	"storj.io/storj/shared/tagsql"
)

// TestWithTx_CommitsEnqueuedWrites checks that writes enqueued with
// EnqueueExec are dispatched and durably committed.
func TestWithTx_CommitsEnqueuedWrites(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_commit")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		var one int
		if err := tx.QueryRowContext(ctx, `SELECT 1`).Scan(&one); err != nil {
			return err
		}
		require.Equal(t, 1, one)

		tx.EnqueueExec(`INSERT INTO t (id) VALUES (1)`)
		tx.EnqueueExec(`INSERT INTO t (id) VALUES (?),(?)`, 2, 3)
		return nil
	})
	require.NoError(t, err)

	var n int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t`).Scan(&n))
	require.Equal(t, 3, n)
}

// TestWithTx_ReadCommittedIsolation checks that the folded prelude actually
// runs the transaction at READ COMMITTED: a row updated and committed by another
// connection after the Tx's first read becomes visible to a later read in
// the same Tx. Under TiDB's default REPEATABLE READ it would stay hidden.
//
// The MySQL @@tx_isolation variable does not reflect a next-transaction-scoped
// SET TRANSACTION, so this asserts the behavior rather than the variable.
func TestWithTx_ReadCommittedIsolation(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_iso")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, v INT)`)
	require.NoError(t, err)
	_, err = testDB.DB.ExecContext(ctx, `INSERT INTO t VALUES (1, 100)`)
	require.NoError(t, err)

	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		var first int
		if err := tx.QueryRowContext(ctx, `SELECT v FROM t WHERE id=1`).Scan(&first); err != nil {
			return err
		}
		require.Equal(t, 100, first)

		// Another connection commits a change after our first read.
		if _, err := testDB.DB.ExecContext(ctx, `UPDATE t SET v=200 WHERE id=1`); err != nil {
			return err
		}

		var second int
		if err := tx.QueryRowContext(ctx, `SELECT v FROM t WHERE id=1`).Scan(&second); err != nil {
			return err
		}
		require.Equal(t, 200, second, "READ COMMITTED must see the other connection's committed update")
		return nil
	})
	require.NoError(t, err)
}

// TestWithTx_CommitWithExec checks that CommitWithExec flushes the buffered
// writes, runs the final statement, and commits — all durably.
func TestWithTx_CommitWithExec(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_cwe")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		tx.EnqueueExec(`INSERT INTO t (id) VALUES (1)`)
		return tx.CommitWithExec(ctx, `INSERT INTO t (id) VALUES (?)`, 2)
	})
	require.NoError(t, err)

	var n int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t`).Scan(&n))
	require.Equal(t, 2, n)
}

// TestWithTx_CommitWithQuery checks that CommitWithQuery flushes the
// buffered writes, passes the final query's rows to scan, and commits durably.
func TestWithTx_CommitWithQuery(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_cwq")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	var total int
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		tx.EnqueueExec(`INSERT INTO t (id) VALUES (1),(2),(3)`)
		return tx.CommitWithQuery(ctx, `SELECT COUNT(*) FROM t`, nil, func(rows tagsql.Rows) error {
			if !rows.Next() {
				return rows.Err()
			}
			return rows.Scan(&total)
		})
	})
	require.NoError(t, err)
	require.Equal(t, 3, total)

	var n int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t`).Scan(&n))
	require.Equal(t, 3, n, "CommitWithQuery must durably commit the flushed writes")
}

// TestWithTx_CommitWithQueryScanError checks that an error returned from
// the scan callback propagates without hanging (Tx owns and closes the
// rows), and that — because COMMIT was already queued in the same stream — the
// flushed writes still commit durably.
func TestWithTx_CommitWithQueryScanError(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_cwq_err")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	sentinel := errFromTest("scan failed")
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		tx.EnqueueExec(`INSERT INTO t (id) VALUES (1)`)
		return tx.CommitWithQuery(ctx, `SELECT id FROM t`, nil, func(rows tagsql.Rows) error {
			return sentinel
		})
	})
	require.ErrorIs(t, err, sentinel)

	var n int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t`).Scan(&n))
	require.Equal(t, 1, n, "COMMIT is queued before scan runs, so the write commits despite the scan error")
}

// TestWithTx_CommitWithQueryScanErrorNotRetried confirms that an error from
// the scanAfterCommit callback is never retried, even when it is otherwise
// classified retryable: the COMMIT was already dispatched, so re-running fn would
// double-apply the committed writes.
func TestWithTx_CommitWithQueryScanErrorNotRetried(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_cwq_noretry")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	// 1213 (deadlock) is retryable per retrydb.ShouldRetry; returning it from the
	// scan callback must still not retry, because the COMMIT already happened.
	retryable := &mysql.MySQLError{Number: 1213, Message: "synthetic deadlock"}
	attempts := 0
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		attempts++ //check-retry:ignore counting attempts across retries is the point of this test
		tx.EnqueueExec(`INSERT INTO t (id) VALUES (1)`)
		return tx.CommitWithQuery(ctx, `SELECT id FROM t`, nil, func(rows tagsql.Rows) error {
			return retryable
		})
	})
	require.Error(t, err)
	require.ErrorIs(t, err, retryable)
	require.Equal(t, 1, attempts, "a scan-callback error must not retry even when retryable")

	var n int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t`).Scan(&n))
	require.Equal(t, 1, n, "the write committed before the scan error and must persist")
}

// TestWithTx_EnqueueExecExpectAffectedCount checks that the affected-row guard
// commits when the count matches and rolls the whole transaction back when it
// does not — the guard runs before COMMIT, in the same round trip — and that the
// error names the desc with the actual and expected counts.
func TestWithTx_EnqueueExecExpectAffectedCount(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_expect")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, v INT)`)
	require.NoError(t, err)
	_, err = testDB.DB.ExecContext(ctx, `INSERT INTO t VALUES (1,0),(2,0)`)
	require.NoError(t, err)

	// Passing case: the UPDATE affects the expected number of rows and a plain
	// (unchecked) insert rides along; both commit.
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		tx.EnqueueExecExpectAffectedCount(2, "objects update", `UPDATE t SET v=1 WHERE id IN (1,2)`)
		tx.EnqueueExec(`INSERT INTO t VALUES (3,9)`)
		return nil
	})
	require.NoError(t, err)

	var updated, inserted int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE v=1`).Scan(&updated))
	require.Equal(t, 2, updated)
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE id=3`).Scan(&inserted))
	require.Equal(t, 1, inserted)

	// Failing case: the UPDATE affects 1 row but 2 were expected, so the guard
	// aborts and nothing commits — including the unchecked insert flushed in the
	// same round trip — and the error names the desc and counts.
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		tx.EnqueueExecExpectAffectedCount(2, "objects update", `UPDATE t SET v=2 WHERE id=1`)
		tx.EnqueueExec(`INSERT INTO t VALUES (4,4)`)
		return nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "objects update")
	require.Contains(t, err.Error(), "affected 1")
	require.Contains(t, err.Error(), "expected 2")

	var changed, leaked int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE v=2`).Scan(&changed))
	require.Equal(t, 0, changed, "failed check must roll back the UPDATE")
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE id=4`).Scan(&leaked))
	require.Equal(t, 0, leaked, "failed check must roll back the sibling insert too")
}

// TestWithTx_MultipleExpectAffectedCount checks that several
// EnqueueExecExpectAffectedCount calls in one transaction each get their own guard:
// when every count matches all writes commit, and when one mismatches the named
// check is the one reported even though it is not the first enqueued write.
func TestWithTx_MultipleExpectAffectedCount(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_multi_expect")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, v INT)`)
	require.NoError(t, err)
	_, err = testDB.DB.ExecContext(ctx, `INSERT INTO t VALUES (1,0),(2,0),(3,0)`)
	require.NoError(t, err)

	// Passing case: two checked writes with different expected counts, plus an
	// unchecked insert between them. Each check captures its own affected count and
	// all of them commit together.
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		tx.EnqueueExecExpectAffectedCount(2, "update pair", `UPDATE t SET v=1 WHERE id IN (1,2)`)
		tx.EnqueueExec(`INSERT INTO t VALUES (4,0)`)
		tx.EnqueueExecExpectAffectedCount(1, "update single", `UPDATE t SET v=1 WHERE id=3`)
		return nil
	})
	require.NoError(t, err)

	var updated, inserted int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE v=1`).Scan(&updated))
	require.Equal(t, 3, updated, "all three checked rows must be updated")
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE id=4`).Scan(&inserted))
	require.Equal(t, 1, inserted, "the unchecked insert between the checks must commit too")

	// Failing case: the first check passes and the second mismatches. The error
	// must name the second check's desc and counts, not the first's, and nothing
	// commits.
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		tx.EnqueueExecExpectAffectedCount(2, "first ok", `UPDATE t SET v=2 WHERE id IN (1,2)`)
		tx.EnqueueExecExpectAffectedCount(2, "second wrong", `UPDATE t SET v=2 WHERE id=3`)
		return nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "second wrong", "the mismatching check must be the one reported")
	require.Contains(t, err.Error(), "affected 1")
	require.Contains(t, err.Error(), "expected 2")
	require.NotContains(t, err.Error(), "first ok", "the passing check must not be reported")

	var changed int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE v=2`).Scan(&changed))
	require.Equal(t, 0, changed, "a single failed check must roll back every write in the batch")

	// Both-fail case: when more than one check mismatches, only the first failing
	// check (in enqueue order) is reported — the guard holds a single message.
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		tx.EnqueueExecExpectAffectedCount(5, "first wrong", `UPDATE t SET v=3 WHERE id=1`)
		tx.EnqueueExecExpectAffectedCount(9, "second wrong", `UPDATE t SET v=3 WHERE id=2`)
		return nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "first wrong", "the first failing check wins the single guard slot")
	require.NotContains(t, err.Error(), "second wrong", "later failing checks are shadowed")
}

// TestWithTx_CommitWithExecFailedCheck ensures a failing EnqueueExecAffected
// check aborts the commit on the CommitWithExec finalizer path too: the check
// runs (via flushVerified) before the folded final-statement;COMMIT, so the
// error propagates and neither the checked write nor the final write commits.
func TestWithTx_CommitWithExecFailedCheck(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_cwe_check")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, v INT)`)
	require.NoError(t, err)
	_, err = testDB.DB.ExecContext(ctx, `INSERT INTO t VALUES (1, 0)`)
	require.NoError(t, err)

	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		// expects 2 rows but the UPDATE affects 1, so the guard aborts.
		tx.EnqueueExecExpectAffectedCount(2, "objects update", `UPDATE t SET v=1 WHERE id=1`)
		return tx.CommitWithExec(ctx, `INSERT INTO t VALUES (2, 2)`)
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "objects update")

	var changed, finalWrite int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE v=1`).Scan(&changed))
	require.Equal(t, 0, changed, "failed check must roll back the enqueued UPDATE")
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE id=2`).Scan(&finalWrite))
	require.Equal(t, 0, finalWrite, "the CommitWithExec final write must not commit when the check fails")
}

// TestWithTx_RepeatableReadOption checks that WithTxOptions folds the
// requested isolation level: under REPEATABLE READ a row committed by another
// connection after the first read stays hidden from a later read.
func TestWithTx_RepeatableReadOption(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_rr")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, v INT)`)
	require.NoError(t, err)
	_, err = testDB.DB.ExecContext(ctx, `INSERT INTO t VALUES (1, 100)`)
	require.NoError(t, err)

	opts := tidbutil.TxOptions{Isolation: sql.LevelRepeatableRead}
	err = tidbutil.WithTxOptions(ctx, testDB.DB, opts, func(ctx context.Context, tx *tidbutil.Tx) error {
		var first int
		if err := tx.QueryRowContext(ctx, `SELECT v FROM t WHERE id=1`).Scan(&first); err != nil {
			return err
		}
		require.Equal(t, 100, first)

		if _, err := testDB.DB.ExecContext(ctx, `UPDATE t SET v=200 WHERE id=1`); err != nil {
			return err
		}

		var second int
		if err := tx.QueryRowContext(ctx, `SELECT v FROM t WHERE id=1`).Scan(&second); err != nil {
			return err
		}
		require.Equal(t, 100, second, "REPEATABLE READ must NOT see the other connection's committed update")
		return nil
	})
	require.NoError(t, err)
}

// TestWithTx_DxDo checks that dx.Do detects the TiDB driver name Tx
// advertises and batches its queries through Tx as a single multi-statement
// round trip, with the isolation+BEGIN prelude folded in front transparently.
func TestWithTx_DxDo(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_dxdo")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE items (id INT PRIMARY KEY, label VARCHAR(64))`)
	require.NoError(t, err)
	_, err = testDB.DB.ExecContext(ctx, `INSERT INTO items VALUES (1,'one'),(2,'two'),(3,'three')`)
	require.NoError(t, err)

	var count int
	var label string
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		// dx.Do is the first operation, so the multi-statement query it issues
		// gets the SET ISOLATION;BEGIN prelude folded in front of it.
		if err := dx.Do(ctx, tx,
			dx.Query{
				Statement: `SELECT COUNT(*) FROM items`,
				Do:        dx.ScanRow(&count),
			},
			dx.Query{
				Statement: `SELECT label FROM items WHERE id = ?`,
				Args:      []any{2},
				Do:        dx.ScanRow(&label),
			},
		); err != nil {
			return err
		}

		// The transaction is open and still usable for enqueued writes.
		tx.EnqueueExec(`INSERT INTO items VALUES (4,'four')`)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 3, count)
	require.Equal(t, "two", label)

	var n int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&n))
	require.Equal(t, 4, n)
}

// TestWithTx_OnlyEnqueue checks the path where fn issues no direct query at
// all: the prelude, the enqueued writes, and COMMIT are all folded into a single
// round trip at commit time.
func TestWithTx_OnlyEnqueue(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_enqueue")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		tx.EnqueueExec(`INSERT INTO t (id) VALUES (1)`)
		tx.EnqueueExec(`INSERT INTO t (id) VALUES (2)`)
		return nil
	})
	require.NoError(t, err)

	var n int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t`).Scan(&n))
	require.Equal(t, 2, n)
}

// TestWithTx_RollbackOnError ensures fn errors trigger a rollback and the
// error reaches the caller. Both a directly-executed write and an enqueued write
// must be discarded.
func TestWithTx_RollbackOnError(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_rb")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	sentinel := errFromTest("rollback please")
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO t (id) VALUES (1)`); err != nil {
			return err
		}
		tx.EnqueueExec(`INSERT INTO t (id) VALUES (2)`)
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)

	var n int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t`).Scan(&n))
	require.Equal(t, 0, n, "rows from rolled-back tx must not be visible")
}

// TestWithTx_RollbackOnCommitFailure exercises the case the commit-probe
// test documents: an enqueued write fails inside the folded "writes;COMMIT"
// batch, so COMMIT never runs and the transaction is left open. WithTx must
// roll it back, discard the earlier write, and leave the connection reusable.
func TestWithTx_RollbackOnCommitFailure(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_commitfail")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)
	// Seed id=2 so the second enqueued INSERT is a duplicate-key error.
	_, err = testDB.DB.ExecContext(ctx, `INSERT INTO t (id) VALUES (2)`)
	require.NoError(t, err)

	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		tx.EnqueueExec(`INSERT INTO t (id) VALUES (1)`)
		tx.EnqueueExec(`INSERT INTO t (id) VALUES (2)`)
		return nil
	})
	require.Error(t, err, "the duplicate-key INSERT must surface as a commit error")

	var hasOne int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE id=1`).Scan(&hasOne))
	require.Equal(t, 0, hasOne, "COMMIT must not have run: id=1 must not be durable")

	// The connection pool must be usable afterwards, proving no dangling
	// transaction was returned to the pool.
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		tx.EnqueueExec(`INSERT INTO t (id) VALUES (3)`)
		return nil
	})
	require.NoError(t, err)

	var hasThree int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE id=3`).Scan(&hasThree))
	require.Equal(t, 1, hasThree)
}

// TestWithTx_RetriesOnRetryableError surfaces a synthetic InnoDB deadlock
// (ER_LOCK_DEADLOCK = 1213) from fn for the first two attempts. WithTx must
// roll back each failing attempt and re-run fn, so only the final commit
// persists. This also confirms the pinned connection's per-statement retry is
// disabled — otherwise the deadlock would be retried at the wrong layer.
func TestWithTx_RetriesOnRetryableError(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_retry")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	const wantAttempts = 3
	attempts := 0
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		attempts++ //check-retry:ignore counting attempts across retries is the point of this test
		if _, err := tx.ExecContext(ctx, `INSERT INTO t (id) VALUES (1)`); err != nil {
			return err
		}
		if attempts < wantAttempts {
			return &mysql.MySQLError{Number: 1213, Message: "synthetic deadlock"}
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, wantAttempts, attempts)

	var n int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t`).Scan(&n))
	require.Equal(t, 1, n, "rolled-back attempts must not persist; only the final commit should")
}

// TestWithTx_DoesNotRetryNonRetryable confirms a non-retryable error from
// fn is returned immediately without re-running fn.
func TestWithTx_DoesNotRetryNonRetryable(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_nonretry")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	attempts := 0
	dup := &mysql.MySQLError{Number: 1062, Message: "synthetic duplicate"}
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		attempts++ //check-retry:ignore counting attempts across retries is the point of this test
		return dup
	})
	require.Error(t, err)
	require.Equal(t, 1, attempts, "non-retryable errors must not retry")
}

// TestWithTx_CommitWithResults checks that CommitWithResults returns one
// result per enqueued write, mapped back across the prelude/guard statements that
// the folded multi-statement injects: the unchecked INSERT's AUTO_INCREMENT id
// and the checked UPDATE's affected-row count.
func TestWithTx_CommitWithResults(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_results")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_INCREMENT, v INT)`)
	require.NoError(t, err)
	// Seed id=10 so the AUTO_INCREMENT insert lands above it and the UPDATE has a
	// row to match.
	_, err = testDB.DB.ExecContext(ctx, `INSERT INTO t (id, v) VALUES (10, 0)`)
	require.NoError(t, err)

	var results []tidbutil.StatementResult
	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		tx.EnqueueExec(`INSERT INTO t (v) VALUES (?)`, 1)
		tx.EnqueueExecExpectAffectedCount(1, "update existing", `UPDATE t SET v=2 WHERE id=10`)
		results, err = tx.CommitWithResults(ctx)
		return err
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	require.Greater(t, results[0].LastInsertID, int64(10), "the insert reports its AUTO_INCREMENT id")
	require.EqualValues(t, 1, results[0].RowsAffected, "the insert affected one row")
	require.EqualValues(t, 1, results[1].RowsAffected, "the update affected one row")

	// Both writes are durable, and the reported insert id addresses the new row.
	var v int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT v FROM t WHERE id=?`, results[0].LastInsertID).Scan(&v))
	require.Equal(t, 1, v)
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT v FROM t WHERE id=10`).Scan(&v))
	require.Equal(t, 2, v)
}

// TestWithTx_CommitWithResultsFailedCheck confirms CommitWithResults still
// enforces the affected-row guard: a mismatch aborts before COMMIT, rolls back,
// and returns no results.
func TestWithTx_CommitWithResultsFailedCheck(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "batchtx_results_check")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, v INT)`)
	require.NoError(t, err)
	_, err = testDB.DB.ExecContext(ctx, `INSERT INTO t (id, v) VALUES (1, 0)`)
	require.NoError(t, err)

	err = tidbutil.WithTx(ctx, testDB.DB, func(ctx context.Context, tx *tidbutil.Tx) error {
		// Expects 2 rows but only id=1 exists, so the guard trips.
		tx.EnqueueExecExpectAffectedCount(2, "update too many", `UPDATE t SET v=9 WHERE id=1`)
		_, err := tx.CommitWithResults(ctx)
		return err
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "update too many")

	var v int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT v FROM t WHERE id=1`).Scan(&v))
	require.Equal(t, 0, v, "the failed-check transaction must have rolled back")
}

type errFromTest string

func (e errFromTest) Error() string { return string(e) }
