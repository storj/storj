// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package postgreskv

import (
	"context"
	"flag"
	"testing"

	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/storage"
	"storj.io/storj/storage/testsuite"
)

var (
	doAltTests = flag.Bool("test-postgreskv-alt", false, "Run the KeyValueStore tests against the alternate PG implementation")
)

func newTestAlternatePostgres(t testing.TB) (store *AlternateClient, cleanup func()) {
	if !*doAltTests {
		t.Skip("alternate-implementation PG tests not enabled.")
	}
	if *pgtest.ConnStr == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", pgtest.DefaultConnStr)
	}

	pgdb, err := AltNew(*pgtest.ConnStr)
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	return pgdb, func() {
		if err := pgdb.Close(); err != nil {
			t.Fatalf("failed to close db: %v", err)
		}
	}
}

func TestSuiteAlt(t *testing.T) {
	store, cleanup := newTestAlternatePostgres(t)
	defer cleanup()

	// zap := zaptest.NewLogger(t)
	// loggedStore := storelogger.New(zap, store)
	testsuite.RunTests(t, store)
}

func BenchmarkSuiteAlt(b *testing.B) {
	store, cleanup := newTestAlternatePostgres(b)
	defer cleanup()

	testsuite.RunBenchmarks(b, store)
}

type pgAltLongBenchmarkStore struct {
	*AlternateClient
}

func (store *pgAltLongBenchmarkStore) BulkImport(ctx context.Context, iter storage.Iterator) error {
	return bulkImport(store.pgConn, iter)
}

func (store *pgAltLongBenchmarkStore) BulkDeleteAll(ctx context.Context) error {
	return bulkDeleteAll(store.pgConn)
}

var _ testsuite.BulkImporter = &pgAltLongBenchmarkStore{}
var _ testsuite.BulkCleaner = &pgAltLongBenchmarkStore{}

func BenchmarkSuiteLongAlt(b *testing.B) {
	store, cleanup := newTestAlternatePostgres(b)
	defer cleanup()

	testsuite.BenchmarkPathOperationsInLargeDb(b, &pgAltLongBenchmarkStore{store})
}
