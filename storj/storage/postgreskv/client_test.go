// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package postgreskv

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/zeebo/errs"

	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/storage"
	"storj.io/storj/storage/testsuite"
)

var ctx = context.Background() // test context

func newTestPostgres(t testing.TB) (store *Client, cleanup func()) {
	if *pgtest.ConnStr == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", pgtest.DefaultConnStr)
	}

	pgdb, err := New(*pgtest.ConnStr)
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

	// zap := zaptest.NewLogger(t)
	// loggedStore := storelogger.New(zap, store)
	testsuite.RunTests(t, store)
}

func TestThatMigrationActuallyHappened(t *testing.T) {
	store, cleanup := newTestPostgres(t)
	defer cleanup()

	rows, err := store.pgConn.Query(`
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

func bulkImport(db *sql.DB, iter storage.Iterator) (err error) {
	txn, err2 := db.Begin()
	if err2 != nil {
		return errs.New("Failed to start transaction: %v", err2)
	}
	defer func() {
		if err == nil {
			err = errs.Combine(err, txn.Commit())
		} else {
			err = errs.Combine(err, txn.Rollback())
		}
	}()

	stmt, err2 := txn.Prepare(pq.CopyIn("pathdata", "bucket", "fullpath", "metadata"))
	if err2 != nil {
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
		if _, err := stmt.Exec([]byte(""), []byte(item.Key), []byte(item.Value)); err != nil {
			return err
		}
	}
	if _, err = stmt.Exec(); err != nil {
		return errs.New("Failed to complete COPY FROM: %v", err)
	}
	return nil
}

func bulkDelete(db *sql.DB) error {
	_, err := db.Exec("TRUNCATE pathdata")
	if err != nil {
		return errs.New("Failed to TRUNCATE pathdata table: %v", err)
	}
	return nil
}

type pgLongBenchmarkStore struct {
	*Client
}

func (store *pgLongBenchmarkStore) BulkImport(iter storage.Iterator) error {
	return bulkImport(store.pgConn, iter)
}

func (store *pgLongBenchmarkStore) BulkDelete() error {
	return bulkDelete(store.pgConn)
}

func BenchmarkSuiteLong(b *testing.B) {
	store, cleanup := newTestPostgres(b)
	defer cleanup()

	testsuite.BenchmarkPathOperationsInLargeDb(b, &pgLongBenchmarkStore{store})
}
