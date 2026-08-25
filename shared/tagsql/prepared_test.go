// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tagsql_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/tagsql"
)

func TestPrepared(t *testing.T) {
	run(t, func(ctx *testcontext.Context, t *testing.T, db tagsql.DB, support tagsql.ContextSupport) {
		_, err := db.ExecContext(ctx, "CREATE TABLE kv (k INT PRIMARY KEY, v INT)")
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, "INSERT INTO kv VALUES (1, 10), (2, 20)")
		require.NoError(t, err)

		placeholder := "?"
		if db.Name() != tagsql.SqliteName {
			placeholder = "$1"
		}
		get := tagsql.Statement("SELECT v FROM kv WHERE k = " + placeholder)
		for i := 0; i < 3; i++ {
			var v int
			require.NoError(t, db.Prepared(get).QueryRowContext(ctx, 2).Scan(&v))
			require.Equal(t, 20, v)
		}

		// A statement that fails to prepare must still serve the plain query
		// error, not something worse, and must not break Close.
		bad := tagsql.Statement("SELECT v FROM missing WHERE k = " + placeholder)
		require.Error(t, db.Prepared(bad).QueryRowContext(ctx, 1).Scan(new(int)))

		rows, err := db.Prepared(get).QueryContext(ctx, 1)
		require.NoError(t, err)
		require.True(t, rows.Next())
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
	})
}
