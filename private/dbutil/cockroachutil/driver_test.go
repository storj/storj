// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package cockroachutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/pgtest"
	"storj.io/storj/private/tagsql"
)

func TestLibPqCompatibility(t *testing.T) {
	connstr := pgtest.PickCockroach(t)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testDB, err := OpenUnique(ctx, connstr, "TestLibPqCompatibility")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)

	// use a single dedicated conn for testing
	conn, err := testDB.Conn(ctx)
	require.NoError(t, err)
	defer ctx.Check(conn.Close)

	// should be in idle status, no transaction, initially
	require.Equal(t, txnStatusIdle, getTxnStatus(ctx, t, conn))
	require.False(t, checkIsInTx(ctx, t, conn))

	// start a transaction
	tx, err := conn.BeginTx(ctx, nil)
	require.NoError(t, err)

	func() {
		defer func() { err = tx.Rollback() }()

		// should be idle in transaction now
		require.Equal(t, txnStatusIdleInTransaction, getTxnStatus(ctx, t, conn))
		require.True(t, checkIsInTx(ctx, t, conn))

		// issue successful query
		rows, err := tx.QueryContext(ctx, `SELECT 1`)
		require.NoError(t, err)

		require.True(t, rows.Next())
		var n int
		err = rows.Scan(&n)
		require.NoError(t, err)
		require.False(t, rows.Next())
		err = rows.Err()
		require.NoError(t, err)
		err = rows.Close()
		require.NoError(t, err)

		// should still be idle in transaction
		require.Equal(t, txnStatusIdleInTransaction, getTxnStatus(ctx, t, conn))
		require.True(t, checkIsInTx(ctx, t, conn))

		// issue bad query
		_, err = tx.QueryContext(ctx, `SELECT BALONEY SANDWICHES`)
		require.Error(t, err)

		// should be in a failed transaction now
		require.Equal(t, txnStatusInFailedTransaction, getTxnStatus(ctx, t, conn))
		require.True(t, checkIsInTx(ctx, t, conn))
	}()

	// check rollback error
	require.NoError(t, err)

	// should be back out of any transaction
	require.Equal(t, txnStatusIdle, getTxnStatus(ctx, t, conn))
	require.False(t, checkIsInTx(ctx, t, conn))
}

func withCockroachConn(ctx context.Context, sqlConn tagsql.Conn, fn func(conn *cockroachConn) error) error {
	return sqlConn.Raw(ctx, func(rawConn interface{}) error {
		crConn, ok := rawConn.(*cockroachConn)
		if !ok {
			return errs.New("conn object is %T, not *cockroachConn", crConn)
		}
		return fn(crConn)
	})
}

func getTxnStatus(ctx context.Context, t *testing.T, sqlConn tagsql.Conn) (txnStatus transactionStatus) {
	err := withCockroachConn(ctx, sqlConn, func(crConn *cockroachConn) error {
		txnStatus = crConn.txnStatus()
		return nil
	})
	require.NoError(t, err)
	return txnStatus
}

func checkIsInTx(ctx context.Context, t *testing.T, sqlConn tagsql.Conn) (isInTx bool) {
	err := withCockroachConn(ctx, sqlConn, func(crConn *cockroachConn) error {
		isInTx = crConn.isInTransaction()
		return nil
	})
	require.NoError(t, err)
	return isInTx
}
