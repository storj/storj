// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package postgreskv

import (
	"flag"
	"testing"

	"go.uber.org/zap/zaptest"

	"storj.io/storj/storage"
	"storj.io/storj/storage/storelogger"
	"storj.io/storj/storage/testsuite"
)

var (
	doAltTests = flag.Bool("test-postgreskv-alt", false, "Run the KeyValueStore tests against the alternate PG implementation")
)

func newTestAlternatePostgres(t testing.TB) (store *AlternateClient, cleanup func()) {
	if !*doAltTests {
		t.Skip("alternate-implementation PG tests not enabled.")
	}
	if *testPostgres == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", defaultPostgresConn)
	}

	pgdb, err := AltNew(*testPostgres)
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

	zap := zaptest.NewLogger(t)
	testsuite.RunTests(t, storelogger.New(zap, store))
}

func BenchmarkSuiteAlt(b *testing.B) {
	store, cleanup := newTestAlternatePostgres(b)
	defer cleanup()

	testsuite.RunBenchmarks(b, store)
}

type pgAltLongBenchmarkStore struct {
	*AlternateClient
}

func (store *pgAltLongBenchmarkStore) BulkImport(iter storage.Iterator) error {
	return bulkImport(store.pgConn, iter)
}

func (store *pgAltLongBenchmarkStore) BulkDelete() error {
	return bulkDelete(store.pgConn)
}

func BenchmarkSuiteLongAlt(b *testing.B) {
	store, cleanup := newTestAlternatePostgres(b)
	defer cleanup()

	testsuite.BenchmarkPathOperationsInLargeDb(b, &pgAltLongBenchmarkStore{store})
}
