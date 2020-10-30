// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package postgreskv

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/pgtest"
	"storj.io/storj/private/tagsql"
	"storj.io/storj/storage"
	"storj.io/storj/storage/testsuite"
)

func openTestPostgres(ctx context.Context, t testing.TB) (store *Client, cleanup func()) {
	connstr := pgtest.PickPostgres(t)

	pgdb, err := Open(ctx, connstr)
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
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, cleanup := openTestPostgres(ctx, t)
	defer cleanup()

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

	store, cleanup := openTestPostgres(ctx, t)
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
	b.Skip("broken")

	ctx := context.Background()

	store, cleanup := openTestPostgres(ctx, b)
	defer cleanup()

	testsuite.RunBenchmarks(b, store)
}

type bulkImportCopyFromSource struct {
	ctx  context.Context
	iter storage.Iterator
	item storage.ListItem
}

func (bs *bulkImportCopyFromSource) Next() bool {
	return bs.iter.Next(bs.ctx, &bs.item)
}

func (bs *bulkImportCopyFromSource) Values() ([]interface{}, error) {
	return []interface{}{bs.item.Key, bs.item.Value}, nil
}

func (bs *bulkImportCopyFromSource) Err() error {
	// we can't determine this from storage.Iterator, I guess
	return nil
}

func bulkImport(ctx context.Context, db tagsql.DB, iter storage.Iterator) (err error) {
	defer mon.Task()(&ctx)(&err)
	pgxConn, err := stdlib.AcquireConn(db.Internal())
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, stdlib.ReleaseConn(db.Internal(), pgxConn))
	}()

	importSource := &bulkImportCopyFromSource{iter: iter}
	_, err = pgxConn.CopyFrom(ctx, pgx.Identifier{"pathdata"}, []string{"fullpath", "metadata"}, importSource)
	return err
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
	ctx := context.Background()

	store, cleanup := openTestPostgres(ctx, b)
	defer cleanup()

	testsuite.BenchmarkPathOperationsInLargeDb(b, &pgLongBenchmarkStore{store})
}
