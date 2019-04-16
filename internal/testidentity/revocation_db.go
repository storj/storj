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
func RevocationDBsTest(t *testing.T, test func(*testing.T, extensions.RevocationDB, storage.KeyValueStore)) {

	t.Run("Redis-backed revocation DB", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		redisServer, err := miniredis.Run()
		require.NoError(t, err)
		defer redisServer.Close()

		{
			// Test using redis-backed revocation DB
			dbURL := "redis://" + redisServer.Addr() + "?db=0"
			redisRevDB, err := identity.NewRevocationDB(dbURL)
			require.NoError(t, err)
			defer ctx.Check(redisRevDB.Close)

			test(t, redisRevDB, redisRevDB.DB)
		}

	})

	t.Run("Bolt-backed revocation DB", func(t *testing.T) {
		{
			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			// Test using bolt-backed revocation DB
			revocationDBPath := ctx.File("revocations.db")

			dbURL := "bolt://" + revocationDBPath
			boltRevDB, err := identity.NewRevocationDB(dbURL)
			require.NoError(t, err)
			defer ctx.Check(boltRevDB.Close)

			test(t, boltRevDB, boltRevDB.DB)
		}
	})

	t.Run("Sqlite-backed revocation DB", func(t *testing.T) {
		{
			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			// Test using bolt-backed revocation DB
			revocationDBPath := ctx.File("revocations.db")

			dbURL := "sqlite://" + revocationDBPath
			boltRevDB, err := identity.NewRevocationDB(dbURL)
			require.NoError(t, err)
			defer ctx.Check(boltRevDB.Close)

			test(t, boltRevDB, boltRevDB.DB)
		}
	})
}
