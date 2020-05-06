// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package postgreskv

import (
	"context"
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/pgtest"
	"storj.io/storj/private/dbutil/txutil"
	"storj.io/storj/private/tagsql"
	"storj.io/storj/storage"
	"storj.io/storj/storage/testsuite"
)

func newTestPostgres(t testing.TB) (store *Client, cleanup func()) {
	connstr := pgtest.PickPostgres(t)

	pgdb, err := New(connstr)
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	return pgdb, func() {
		if err := pgdb.Close(); err != nil {
			t.Fatalf("failed to close db: %v", err)
		}
	}
}

func TestSuite(t *testing.T) {
	store, cleanup := newTestPostgres(t)
	defer cleanup()

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	err := store.MigrateToLatest(ctx)
	require.NoError(t, err)

	// zap := zaptest.NewLogger(t)
	// loggedStore := storelogger.New(zap, store)
	store.SetLookupLimit(500)
	testsuite.RunTests(t, store)
}

func TestThatMigrationActuallyHappened(t *testing.T) {
	t.Skip()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, cleanup := newTestPostgres(t)
	defer cleanup()

	rows, err := store.db.Query(ctx, `
		SELECT prosrc
		  FROM pg_catalog.pg_proc p,
		       pg_catalog.pg_namespace n
		 WHERE p.pronamespace = n.oid
		       AND p.proname = 'list_directory'
		       AND n.nspname = ANY(current_schemas(true))
		       AND p.pronargs = 4
	`)
	if err != nil {
		t.Fatalf("failed to get list_directory source: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Fatalf("failed to close rows: %v", err)
		}
	}()

	numFound := 0
	for rows.Next() {
		numFound++
		if numFound > 1 {
			t.Fatal("there are multiple eligible list_directory() functions??")
		}
		var source string
		if err := rows.Scan(&source); err != nil {
			t.Fatalf("failed to read list_directory source: %v", err)
		}
		if strings.Contains(source, "distinct_prefix (truncatedpath)") {
			t.Fatal("list_directory() function in pg appears to be the oldnbusted one")
		}
	}
}

func BenchmarkSuite(b *testing.B) {
	store, cleanup := newTestPostgres(b)
	defer cleanup()

	testsuite.RunBenchmarks(b, store)
}

func bulkImport(ctx context.Context, db tagsql.DB, iter storage.Iterator) error {
	return txutil.WithTx(ctx, db, nil, func(ctx context.Context, txn tagsql.Tx) (err error) {
		stmt, err := txn.Prepare(ctx, pq.CopyIn("pathdata", "fullpath", "metadata"))
		if err != nil {
			return errs.New("Failed to initialize COPY FROM: %v", err)
		}
		defer func() {
			err2 := stmt.Close()
			if err2 != nil {
				err = errs.Combine(err, errs.New("Failed to close COPY FROM statement: %v", err2))
			}
		}()

		var item storage.ListItem
		for iter.Next(ctx, &item) {
			if _, err := stmt.Exec(ctx, []byte(item.Key), []byte(item.Value)); err != nil {
				return err
			}
		}
		if _, err = stmt.Exec(ctx); err != nil {
			return errs.New("Failed to complete COPY FROM: %v", err)
		}
		return nil
	})
}

func bulkDeleteAll(ctx context.Context, db tagsql.DB) error {
	_, err := db.Exec(ctx, "TRUNCATE pathdata")
	if err != nil {
		return errs.New("Failed to TRUNCATE pathdata table: %v", err)
	}
	return nil
}

type pgLongBenchmarkStore struct {
	*Client
}

func (store *pgLongBenchmarkStore) BulkImport(ctx context.Context, iter storage.Iterator) error {
	return bulkImport(ctx, store.db, iter)
}

func (store *pgLongBenchmarkStore) BulkDeleteAll(ctx context.Context) error {
	return bulkDeleteAll(ctx, store.db)
}

func BenchmarkSuiteLong(b *testing.B) {
	store, cleanup := newTestPostgres(b)
	defer cleanup()

	testsuite.BenchmarkPathOperationsInLargeDb(b, &pgLongBenchmarkStore{store})
}
