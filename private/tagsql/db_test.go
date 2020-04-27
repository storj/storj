// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package tagsql_test

import (
	"testing"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/cockroachutil"
	"storj.io/storj/private/dbutil/pgtest"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/tagsql"
)

func run(t *testing.T, fn func(*testcontext.Context, *testing.T, tagsql.DB, tagsql.ContextSupport)) {
	t.Helper()

	t.Run("mattn-sqlite3", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		db, err := tagsql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Check(db.Close)

		fn(ctx, t, db, tagsql.SupportBasic)
	})

	t.Run("lib-pq-postgres", func(t *testing.T) {
		connstr := pgtest.PickPostgres(t)

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		db, err := pgutil.OpenUnique(ctx, connstr, "detect")
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		fn(ctx, t, db.DB, tagsql.SupportNone)
	})

	t.Run("lib-pq-cockroach", func(t *testing.T) {
		connstr := pgtest.PickCockroach(t)

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		db, err := cockroachutil.OpenUnique(ctx, connstr, "detect")
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		fn(ctx, t, db.DB, tagsql.SupportNone)
	})
}
