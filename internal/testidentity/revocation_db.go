// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testidentity

import (
	"testing"

	"github.com/alicebob/miniredis"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/storage"
)

// RevocationDBsTest runs the passed test function with each type of revocation database.
func RevocationDBsTest(ctx *testcontext.Context, t *testing.T, test func(*testing.T, extensions.RevocationDB, storage.KeyValueStore)) {
	revocationDBPath := ctx.File("revocations.db")

	t.Run("Redis-backed revocation DB", func(t *testing.T) {
		redisServer, err := miniredis.Run()
		require.NoError(t, err)

		{
			// Test using redis-backed revocation DB
			dbURL := "redis://" + redisServer.Addr() + "?db=0"
			redisRevDB, err := identity.NewRevDB(dbURL)
			require.NoError(t, err)

			test(t, redisRevDB, redisRevDB.DB)
			ctx.Check(redisRevDB.Close)
		}

		redisServer.Close()
	})

	t.Run("Bolt-backed revocation DB", func(t *testing.T) {
		{
			// Test using bolt-backed revocation DB
			dbURL := "bolt://" + revocationDBPath
			boltRevDB, err := identity.NewRevDB(dbURL)
			require.NoError(t, err)

			test(t, boltRevDB, boltRevDB.DB)
			ctx.Check(boltRevDB.Close)
		}
	})
}
