// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
package cockroachkv

import (
	"testing"

	_ "github.com/lib/pq"

	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/storage/testsuite"
)

func newTestCockroachDB(t testing.TB) (store *Client, cleanup func()) {
	if *pgtest.CrdbConnStr == "" {
		t.Skipf("postgres flag missing, example:\n-cockroach-test-db=%s", pgtest.DefaultCrdbConnStr)
	}

	crdb, err := New(*pgtest.CrdbConnStr)
	if err != nil {
		t.Fatalf("init: %+v", err)
	}

	return crdb, func() {
		if err := crdb.Close(); err != nil {
			t.Fatalf("failed to close db: %v", err)
		}
	}
}

func TestSuite(t *testing.T) {
	store, cleanup := newTestCockroachDB(t)
	defer cleanup()

	testsuite.RunTests(t, store)
}
