// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testrevocation

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/peertls/extensions"
	"storj.io/common/testcontext"
	"storj.io/storj/private/kvstore"
	"storj.io/storj/private/revocation"
	"storj.io/storj/private/testredis"
)

// RunDBs runs the passed test function with each type of revocation database.
func RunDBs(t *testing.T, test func(*testing.T, extensions.RevocationDB, kvstore.Store)) {
	t.Run("Redis", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		redis, err := testredis.Mini(ctx)
		require.NoError(t, err)
		defer ctx.Check(redis.Close)

		// Test using redis-backed revocation DB
		dbURL := "redis://" + redis.Addr() + "?db=0"
		db, err := revocation.OpenDB(ctx, dbURL)
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		test(t, db, db.TestGetStore())
	})

	t.Run("Bolt", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		// Test using bolt-backed revocation DB
		db, err := revocation.OpenDB(ctx, "bolt://"+ctx.File("revocations.db"))
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		test(t, db, db.TestGetStore())
	})
}
