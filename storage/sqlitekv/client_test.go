// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqlitekv

import (
	"context"
	"testing"

	"storj.io/storj/storage/testsuite"
)

var ctx = context.Background() // test context

func newTestSqlite(t testing.TB) (store *Client, cleanup func()) {
	sqliteConn, err := New("file::memory:?mode=memory")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	return sqliteConn, func() {
		if err := sqliteConn.Close(); err != nil {
			t.Fatalf("failed to close db: %v", err)
		}
	}
}

func TestSuite(t *testing.T) {
	store, cleanup := newTestSqlite(t)
	defer cleanup()

	testsuite.RunTests(t, store)
}

func BenchmarkSuite(b *testing.B) {
	store, cleanup := newTestSqlite(b)
	defer cleanup()

	testsuite.RunBenchmarks(b, store)
}