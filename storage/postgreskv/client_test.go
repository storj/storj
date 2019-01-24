// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package postgreskv

import (
	"database/sql"
	"flag"
	"os"
	"testing"

	"github.com/lib/pq"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/storage"
	"storj.io/storj/storage/storelogger"
	"storj.io/storj/storage/testsuite"
)

const (
	// this connstring is expected to work under the storj-test docker-compose instance
	defaultPostgresConn = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"
)

var (
	testPostgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRES_TEST"), "PostgreSQL test database connection string")
)

func newTestPostgres(t testing.TB) (store *Client, cleanup func()) {
	if *testPostgres == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", defaultPostgresConn)
	}

	pgdb, err := New(*testPostgres)
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

	zap := zaptest.NewLogger(t)
	testsuite.RunTests(t, storelogger.New(zap, store))
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
	for iter.Next(&item) {
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
