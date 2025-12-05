// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil_test

import (
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/tempdb"
)

func TestExplain(t *testing.T) {
	dbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, connstr string) {
		db, err := tempdb.OpenUnique(ctx, zaptest.NewLogger(t), connstr, "explain", nil)
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		_, err = db.DB.ExecContext(ctx, `CREATE TABLE people ( name TEXT, PRIMARY KEY (name) )`)
		require.NoError(t, err)

		exp, err := pgutil.Explain(ctx, db.DB, "SELECT * FROM people WHERE name = $1", "user")
		require.NoError(t, err)

		t.Logf("%v", exp)
	})
}

func TestRoughInlinePlaceholders(t *testing.T) {
	type customStringType string
	var hello customStringType = "he'llo"

	s, err := pgutil.RoughInlinePlaceholders("SELECT $1, $2, $3", 5, uuid.UUID{}, hello)
	require.NoError(t, err)
	require.Equal(t, `SELECT 5, '\x00000000000000000000000000000000', 'he''llo'`, s)
}
