// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package dx_test

import (
	"context"
	"database/sql"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/dx"
	"storj.io/storj/shared/dbutil/tempdb"
)

func TestDo(t *testing.T) {
	t.Parallel()
	for _, sat := range satellitedbtest.Databases(t) {
		t.Run(sat.Name, func(t *testing.T) {
			t.Parallel()

			if sat.MetabaseDB.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", sat.MetabaseDB.Name, sat.MetabaseDB.Message)
			}

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			log := zaptest.NewLogger(t)
			tempDB, err := tempdb.OpenUnique(ctx, log, sat.MetabaseDB.URL, "dx_test", sat.MetabaseDB.ExtraStatements)
			require.NoError(t, err)
			defer ctx.Check(tempDB.Close)

			runDoTests(ctx, t, tempDB)
		})
	}
}

func runDoTests(ctx context.Context, t *testing.T, tempDB *dbutil.TempDatabase) {
	db := tempDB.DB
	ph := placeholderFor(tempDB.Implementation)

	createItems := `CREATE TABLE items (id ` + bigintType(tempDB.Implementation) + ` NOT NULL, label ` + textType(tempDB.Implementation) + ` NOT NULL, PRIMARY KEY (id))`
	if tempDB.Implementation == dbutil.Spanner {
		// Spanner places PRIMARY KEY outside the column list.
		createItems = `CREATE TABLE items (id INT64 NOT NULL, label STRING(64) NOT NULL) PRIMARY KEY (id)`
	}
	_, err := db.ExecContext(ctx, createItems)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO items (id, label) VALUES (`+ph(1)+`,`+ph(2)+`),(`+ph(3)+`,`+ph(4)+`),(`+ph(5)+`,`+ph(6)+`)`,
		int64(1), "one",
		int64(2), "two",
		int64(3), "three",
	)
	require.NoError(t, err)

	t.Run("multiple statements walk in order", func(t *testing.T) {
		var count int
		var first string
		var ids []int64

		err := dx.Do(ctx, db,
			dx.Query{
				Statement: `SELECT COUNT(*) FROM items`,
				Do: func(rows dx.Rows) error {
					require.True(t, rows.Next())
					return rows.Scan(&count)
				},
			},
			dx.Query{
				Statement: `SELECT label FROM items WHERE id = ` + ph(1),
				Args:      []any{int64(1)},
				Do: func(rows dx.Rows) error {
					require.True(t, rows.Next())
					return rows.Scan(&first)
				},
			},
			dx.Query{
				Statement: `SELECT id FROM items ORDER BY id`,
				Do: func(rows dx.Rows) error {
					for rows.Next() {
						var id int64
						if err := rows.Scan(&id); err != nil {
							return err
						}
						ids = append(ids, id)
					}
					return nil
				},
			},
		)
		require.NoError(t, err)
		require.Equal(t, 3, count)
		require.Equal(t, "one", first)
		require.Equal(t, []int64{1, 2, 3}, ids)
	})

	t.Run("empty queries are skipped", func(t *testing.T) {
		var count int

		err := dx.Do(ctx, db,
			dx.Query{},
			dx.Query{
				Statement: `SELECT COUNT(*) FROM items`,
				Do: func(rows dx.Rows) error {
					require.True(t, rows.Next())
					return rows.Scan(&count)
				},
			},
			dx.Query{},
		)
		require.NoError(t, err)
		require.Equal(t, 3, count)
	})

	t.Run("all empty queries no-op", func(t *testing.T) {
		require.NoError(t, dx.Do(ctx, db))
		require.NoError(t, dx.Do(ctx, db, dx.Query{}, dx.Query{}))
	})

	t.Run("runs inside a transaction", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		var count int
		var label string
		err = dx.Do(ctx, tx,
			dx.Query{
				Statement: `SELECT COUNT(*) FROM items`,
				Do: func(rows dx.Rows) error {
					require.True(t, rows.Next())
					return rows.Scan(&count)
				},
			},
			dx.Query{
				Statement: `SELECT label FROM items WHERE id = ` + ph(1),
				Args:      []any{int64(2)},
				Do: func(rows dx.Rows) error {
					require.True(t, rows.Next())
					return rows.Scan(&label)
				},
			},
		)
		require.NoError(t, err)
		require.Equal(t, 3, count)
		require.Equal(t, "two", label)
		require.NoError(t, tx.Rollback())
	})

	t.Run("ScanRow", func(t *testing.T) {
		var label string
		err := dx.Do(ctx, db,
			dx.Query{
				Statement: `SELECT label FROM items WHERE id = ` + ph(1),
				Args:      []any{int64(2)},
				Do:        dx.ScanRow(&label),
			},
		)
		require.NoError(t, err)
		require.Equal(t, "two", label)
	})

	t.Run("ScanRow returns ErrNoRows when empty", func(t *testing.T) {
		var label string
		err := dx.Do(ctx, db,
			dx.Query{
				Statement: `SELECT label FROM items WHERE id = ` + ph(1),
				Args:      []any{int64(999)},
				Do:        dx.ScanRow(&label),
			},
		)
		require.ErrorIs(t, err, sql.ErrNoRows)
		require.Equal(t, "", label)
	})

	t.Run("ScanRowOptional", func(t *testing.T) {
		var label string
		err := dx.Do(ctx, db,
			dx.Query{
				Statement: `SELECT label FROM items WHERE id = ` + ph(1),
				Args:      []any{int64(3)},
				Do:        dx.ScanRowOptional(&label),
			},
		)
		require.NoError(t, err)
		require.Equal(t, "three", label)
	})

	t.Run("ScanRowOptional leaves dest untouched when empty", func(t *testing.T) {
		label := "untouched"
		err := dx.Do(ctx, db,
			dx.Query{
				Statement: `SELECT label FROM items WHERE id = ` + ph(1),
				Args:      []any{int64(999)},
				Do:        dx.ScanRowOptional(&label),
			},
		)
		require.NoError(t, err)
		require.Equal(t, "untouched", label)
	})

	t.Run("Do callback error propagates", func(t *testing.T) {
		sentinel := errInjected{}
		err := dx.Do(ctx, db,
			dx.Query{
				Statement: `SELECT id FROM items`,
				Do: func(rows dx.Rows) error {
					return sentinel
				},
			},
		)
		require.ErrorIs(t, err, sentinel)
	})

	t.Run("WithRows iterates", func(t *testing.T) {
		var ids []int64
		err := dx.WithRows(db.QueryContext(ctx, `SELECT id FROM items ORDER BY id`))(func(rows dx.Rows) error {
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err != nil {
					return err
				}
				ids = append(ids, id)
			}
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, []int64{1, 2, 3}, ids)
	})

	t.Run("WithRows propagates query error", func(t *testing.T) {
		sentinel := errInjected{}
		err := dx.WithRows(nil, sentinel)(func(rows dx.Rows) error {
			t.Fatal("callback must not be invoked when err is non-nil")
			return nil
		})
		require.ErrorIs(t, err, sentinel)
	})

	t.Run("WithRows propagates callback error", func(t *testing.T) {
		sentinel := errInjected{}
		err := dx.WithRows(db.QueryContext(ctx, `SELECT id FROM items`))(func(rows dx.Rows) error {
			return sentinel
		})
		require.ErrorIs(t, err, sentinel)
	})

	t.Run("Tx advertises driver name for batching", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		require.Equal(t, db.Name(), tx.Name())

		var count int
		var first string
		err = dx.Do(ctx, tx,
			dx.Query{
				Statement: `SELECT COUNT(*) FROM items`,
				Do:        dx.ScanRow(&count),
			},
			dx.Query{
				Statement: `SELECT label FROM items WHERE id = ` + ph(1),
				Args:      []any{int64(1)},
				Do:        dx.ScanRow(&first),
			},
		)
		require.NoError(t, err)
		require.Equal(t, 3, count)
		require.Equal(t, "one", first)
	})
}

type errInjected struct{}

func (errInjected) Error() string { return "injected" }

func placeholderFor(impl dbutil.Implementation) func(int) string {
	switch impl {
	case dbutil.Postgres, dbutil.Cockroach:
		return func(i int) string { return "$" + strconv.Itoa(i) }
	case dbutil.Spanner:
		return func(i int) string { return "@p" + strconv.Itoa(i) }
	default:
		return func(int) string { return "?" }
	}
}

func bigintType(impl dbutil.Implementation) string {
	if impl == dbutil.Spanner {
		return "INT64"
	}
	return "BIGINT"
}

func textType(impl dbutil.Implementation) string {
	if impl == dbutil.Spanner {
		return "STRING(64)"
	}
	return "VARCHAR(64)"
}
