// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
package cockroachkv

import (
	"context"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/cockroachutil"
	"storj.io/storj/private/dbutil/pgtest"
	"storj.io/storj/storage/testsuite"
)

func newTestCockroachDB(ctx context.Context, t testing.TB) (store *Client, cleanup func()) {
	connstr := pgtest.PickCockroach(t)

	tdb, err := cockroachutil.OpenUnique(ctx, connstr, "test-schema")
	if err != nil {
		t.Fatalf("init: %+v", err)
	}

	return NewWith(tdb.DB, connstr), func() {
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

	err := store.MigrateToLatest(ctx)
	require.NoError(t, err)

	store.SetLookupLimit(500)
	testsuite.RunTests(t, store)
}
