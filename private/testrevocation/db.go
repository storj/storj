// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testrevocation

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/peertls/extensions"
	"storj.io/common/testcontext"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/storage"
	"storj.io/storj/storage/redis/redisserver"
)

// RunDBs runs the passed test function with each type of revocation database.
func RunDBs(t *testing.T, test func(*testing.T, extensions.RevocationDB, storage.KeyValueStore)) {
	t.Run("Redis", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		addr, cleanup, err := redisserver.Start()
		require.NoError(t, err)
		defer cleanup()

		// Test using redis-backed revocation DB
		dbURL := "redis://" + addr + "?db=0"
		db, err := revocation.NewDB(dbURL)
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		test(t, db, db.TestGetStore())
	})

	t.Run("Bolt", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		// Test using bolt-backed revocation DB
		db, err := revocation.NewDB("bolt://" + ctx.File("revocations.db"))
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		test(t, db, db.TestGetStore())
	})
}
