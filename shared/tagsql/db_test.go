// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package tagsql_test

import (
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil/cockroachutil"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/tagsql"
)

func run(t *testing.T, fn func(*testcontext.Context, *testing.T, tagsql.DB, tagsql.ContextSupport)) {
	t.Helper()

	t.Run("mattn-sqlite3", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Check(db.Close)

		fn(ctx, t, db, tagsql.SupportBasic)
	})

	t.Run("jackc-pgx-postgres", func(t *testing.T) {
		connstr := dbtest.PickPostgres(t)

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		db, err := pgutil.OpenUnique(ctx, connstr, "detect")
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		db.SetMaxOpenConns(100)
		db.SetMaxIdleConns(100)

		fn(ctx, t, db.DB, tagsql.SupportAll)
	})

	t.Run("jackc-pgx-cockroach", func(t *testing.T) {
		connstr := dbtest.PickCockroach(t)

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		db, err := cockroachutil.OpenUnique(ctx, connstr, "detect")
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		db.SetMaxOpenConns(100)
		db.SetMaxIdleConns(100)

		fn(ctx, t, db.DB, tagsql.SupportAll)
	})
}
