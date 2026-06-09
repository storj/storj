// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/tidbutil"
)

// TestMultiStatementStopsOnError documents how TiDB handles a multi-statement
// query of the form "BEGIN; INSERT ...; <failing stmt>; COMMIT" sent in a single
// round trip:
//
//   - Execution stops at the failing statement and the error is returned.
//   - COMMIT never runs, so earlier writes are NOT durably committed.
//   - BUT the transaction started by BEGIN is left OPEN on the connection; the
//     earlier writes sit uncommitted (and hold locks) until the connection is
//     explicitly rolled back. The error does not auto-rollback.
func TestMultiStatementStopsOnError(t *testing.T) {
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")
	ctx := testcontext.New(t)
	testDB, err := tidbutil.OpenUnique(ctx, connstr, "commitfold")
	require.NoError(t, err)
	defer ctx.Check(testDB.Close)
	db := testDB.DB

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY)`)
	require.NoError(t, err)
	// Seed id=2 so a second INSERT of id=2 is a runtime duplicate-key error.
	_, err = db.ExecContext(ctx, `INSERT INTO t (id) VALUES (2)`)
	require.NoError(t, err)

	// Pin one physical connection so transaction state is observed
	// deterministically rather than across arbitrary pooled connections.
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer ctx.Check(conn.Close)

	// id=1 is valid; the following INSERT of id=2 fails on the primary key; the
	// COMMIT comes after the failing statement.
	_, err = conn.ExecContext(ctx,
		`BEGIN; INSERT INTO t (id) VALUES (1); INSERT INTO t (id) VALUES (2); COMMIT`)
	require.Error(t, err, "the duplicate-key INSERT must surface as an error")

	// On the SAME connection the transaction is still open, so the uncommitted
	// id=1 is visible to this session: the BEGIN was not auto-rolled-back.
	var visibleInOpenTxn int
	require.NoError(t, conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE id=1`).Scan(&visibleInOpenTxn))
	require.Equal(t, 1, visibleInOpenTxn, "id=1 should be visible within the still-open transaction")

	// Explicitly end the dangling transaction. Because COMMIT never executed,
	// id=1 was never durable and disappears on ROLLBACK.
	_, err = conn.ExecContext(ctx, `ROLLBACK`)
	require.NoError(t, err)

	var durable int
	require.NoError(t, conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM t WHERE id=1`).Scan(&durable))
	require.Equal(t, 0, durable, "COMMIT must not have executed: id=1 must not be durably committed")
}
