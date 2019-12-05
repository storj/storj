// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
package cockroachkv

import (
	"testing"

	_ "github.com/lib/pq"

	"storj.io/storj/private/dbutil/cockroachutil"
	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/storage/cockroachkv/schema"
	"storj.io/storj/storage/testsuite"
)

func newTestCockroachDB(t testing.TB) (store *Client, cleanup func()) {
	if *pgtest.CrdbConnStr == "" {
		t.Skipf("cockroach flag missing, example:\n-cockroach-test-db=%s", pgtest.DefaultCrdbConnStr)
	}

	tdb, err := cockroachutil.OpenUnique(*pgtest.CrdbConnStr, "test-schema")
	if err != nil {
		t.Fatalf("init: %+v", err)
	}

	err = schema.PrepareDB(tdb.DB)
	if err != nil {
		t.Fatalf("init: %+v", err)
	}

	return NewWith(tdb.DB), func() {
		if err := tdb.Close(); err != nil {
			t.Fatalf("failed to close db: %v", err)
		}
	}
}

func TestSuite(t *testing.T) {
	store, cleanup := newTestCockroachDB(t)
	defer cleanup()

	testsuite.RunTests(t, store)
}
