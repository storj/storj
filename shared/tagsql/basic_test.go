// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package tagsql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/tagsql"
)

func TestDetect(t *testing.T) {
	run(t, func(parentctx *testcontext.Context, t *testing.T, db tagsql.DB, support tagsql.ContextSupport) {
		_, err := db.ExecContext(parentctx, "CREATE TABLE example (num INT)")
		require.NoError(t, err)
		_, err = db.ExecContext(parentctx, "INSERT INTO example (num) values (1)")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(parentctx)
		cancel()

		var verify func(t require.TestingT, err error, msgAndArgs ...interface{})
		if support.Basic() {
			verify = require.Error
		} else {
			verify = require.NoError
		}

		err = db.PingContext(ctx)
		verify(t, err)

		_, err = db.ExecContext(ctx, "INSERT INTO example (num) values (1)")
		verify(t, err)

		row := db.QueryRowContext(ctx, "select num from example")
		var value int64
		err = row.Scan(&value)
		verify(t, err)

		var rows tagsql.Rows
		rows, err = db.QueryContext(ctx, "select num from example")
		verify(t, err)
		if rows != nil {
			require.NoError(t, rows.Err())
			require.NoError(t, rows.Close())
		}

		if support.Transactions() {
			tx, err := db.BeginTx(ctx, nil)
			require.Error(t, err)
			if tx != nil {
				require.NoError(t, tx.Rollback())
			}
		}

		var verifyTx func(t require.TestingT, err error, msgAndArgs ...interface{})
		if support.Transactions() {
			verifyTx = require.Error
		} else {
			verifyTx = require.NoError
		}

		tx, err := db.BeginTx(parentctx, nil)
		require.NoError(t, err)

		_, err = tx.ExecContext(ctx, "INSERT INTO example (num) values (1)")
		verifyTx(t, err)

		rows, err = tx.QueryContext(ctx, "select num from example")
		verifyTx(t, err)
		if rows != nil {
			require.NoError(t, rows.Err())
			require.NoError(t, rows.Close())
		}

		row = tx.QueryRowContext(ctx, "select num from example")
		var value2 int64
		// lib/pq seems to stall here for some reason?
		err = row.Scan(&value2)
		verifyTx(t, err)

		require.NoError(t, tx.Commit())
	})
}
