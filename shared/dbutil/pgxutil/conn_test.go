// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package pgxutil_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/pgxutil"
	"storj.io/storj/shared/dbutil/tempdb"
)

func TestConn(t *testing.T) {
	dbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, connstr string) {
		db, err := tempdb.OpenUnique(ctx, zaptest.NewLogger(t), connstr, "pgutil-query", nil)
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		require.NoError(t,
			pgxutil.Conn(ctx, db.DB, func(conn *pgx.Conn) error {
				return nil
			}))

		require.Error(t,
			pgxutil.Conn(ctx, db.DB, func(conn *pgx.Conn) error {
				return errors.New("xyz")
			}))
	})
}
