// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
package cockroachkv

import (
	"context"
	"testing"

	_ "github.com/lib/pq"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/cockroachutil"
	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/storage/cockroachkv/schema"
	"storj.io/storj/storage/testsuite"
)

func newTestCockroachDB(ctx context.Context, t testing.TB) (store *Client, cleanup func()) {
	if *pgtest.CrdbConnStr == "" {
		t.Skipf("cockroach flag missing, example:\n-cockroach-test-db=%s", pgtest.DefaultCrdbConnStr)
	}

	tdb, err := cockroachutil.OpenUnique(ctx, *pgtest.CrdbConnStr, "test-schema")
	if err != nil {
		t.Fatalf("init: %+v", err)
	}

	err = schema.PrepareDB(ctx, tdb.DB)
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
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, cleanup := newTestCockroachDB(ctx, t)
	defer cleanup()

	testsuite.RunTests(t, store)
}
