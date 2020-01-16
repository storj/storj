// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
package postgreskv

import (
	"context"
	"testing"

	_ "github.com/lib/pq"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/private/dbutil/tempdb"
	"storj.io/storj/storage/postgreskv/schema"
	"storj.io/storj/storage/testsuite"
)

func newTestPostgresKV2DB(ctx *testcontext.Context, t testing.TB) (store *Client, cleanup func()) {
	if *pgtest.ConnStr == "" {
		t.Skipf("postgresql flag missing, example:\n-postgres-test-db=%s", pgtest.DefaultConnStr)
	}

	tdb, err := tempdb.OpenUnique(ctx, *pgtest.ConnStr, "test-schema")
	if err != nil {
		t.Fatalf("init: %+v", err)
	}

	err = schema.PrepareDB(context.TODO(), tdb.DB, *pgtest.ConnStr)
	if err != nil {
		t.Fatalf("init: %+v", err)
	}

	return NewWith(tdb.DB), func() {
		ctx.Check(tdb.Close)
	}
}

func TestSuite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, cleanup := newTestPostgresKV2DB(ctx, t)
	defer cleanup()

	testsuite.RunTests(t, store)
}
