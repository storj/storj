// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/tidbutil"
)

// TestWithRawTx_AllRowsAffected runs a multi-statement DELETE/INSERT inside
// WithRawTx and asserts AllRowsAffected returns a per-statement count. This is
// the property database/sql.driverResult hides and that WithRawTx exists to
// surface.
func TestWithRawTx_AllRowsAffected(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "rawtx_arc")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	err = tidbutil.WithRawTx(ctx, testDB.DB, func(ctx context.Context, tx tidbutil.RawTx) error {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO t (id) VALUES (1),(2),(3);
			INSERT INTO t (id) VALUES (4),(5);
			DELETE FROM t WHERE id IN (1,4);
		`)
		if err != nil {
			return err
		}
		counts := res.AllRowsAffected()
		require.Equal(t, []int64{3, 2, 2}, counts)
		return nil
	})
	require.NoError(t, err)

	var n int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t`).Scan(&n))
	require.Equal(t, 3, n)
}

// TestWithRawTx_RollbackOnError ensures fn errors trigger a rollback and the
// error is propagated to the caller without wrapping that obscures it.
func TestWithRawTx_RollbackOnError(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "rawtx_rb")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	sentinel := errFromTest("rollback please")
	err = tidbutil.WithRawTx(ctx, testDB.DB, func(ctx context.Context, tx tidbutil.RawTx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO t (id) VALUES (1)`)
		if err != nil {
			return err
		}
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)

	var n int
	require.NoError(t, testDB.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM t`).Scan(&n))
	require.Equal(t, 0, n, "rows from rolled-back tx must not be visible")
}

// TestWithRawTx_RetriesOnRetryableError surfaces a synthetic InnoDB deadlock
// (ER_LOCK_DEADLOCK = 1213) from fn for the first two attempts. WithRawTx must
// roll back each failing attempt and run fn again, so only the final INSERT
// persists.
func TestWithRawTx_RetriesOnRetryableError(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "rawtx_retry")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	const wantAttempts = 3
	attempts := 0
	err = tidbutil.WithRawTx(ctx, testDB.DB, func(ctx context.Context, tx tidbutil.RawTx) error {
		attempts++
		_, err := tx.ExecContext(ctx, `INSERT INTO t (id) VALUES (1)`)
		if err != nil {
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

// TestWithRawTx_DoesNotRetryNonRetryable confirms WithRawTx returns the first
// non-retryable error from fn immediately without re-running fn. Pairs with
// TestWithRawTx_RetriesOnRetryableError to pin both branches of the retry
// classifier.
func TestWithRawTx_DoesNotRetryNonRetryable(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	testDB, err := tidbutil.OpenUnique(ctx, connstr, "rawtx_nonretry")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	_, err = testDB.DB.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	// 1062 is ER_DUP_ENTRY — surfaced from a real INSERT below; not in the
	// retrydb allowlist.
	attempts := 0
	dup := &mysql.MySQLError{Number: 1062, Message: "synthetic duplicate"}
	err = tidbutil.WithRawTx(ctx, testDB.DB, func(ctx context.Context, tx tidbutil.RawTx) error {
		attempts++
		return dup
	})
	require.Error(t, err)
	var myErr *mysql.MySQLError
	require.True(t, errors.As(err, &myErr), "expected MySQLError, got %T: %v", err, err)
	require.Equal(t, uint16(1062), myErr.Number)
	require.Equal(t, 1, attempts, "non-retryable errors must not retry")
}

type errFromTest string

func (e errFromTest) Error() string { return string(e) }
