// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package collector_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
)

func TestCollector(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		StorageNodeCount: 4, SatelliteCount: 1, UplinkCount: 1, MultinodeCount: 0,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
				config.Collector.Interval = -1
			},
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 2, 4, 4),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// disableMigrationBackend disables the migration backend for
		// this test to ensure that pieces with TTL are not moved to the
		// hashstore, allowing the test to focus on the collection of
		// expired pieces in the old store. This is necessary because
		// the test is specifically designed to verify the functionality
		// of the old collector.
		disableMigrationBackend(ctx, planet)

		// check that the collector runs without error when there are no expired pieces
		err := planet.StorageNodes[1].StorageOld.Collector.Collect(ctx, time.Now())
		require.NoError(t, err)

		uplink := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		data := testrand.Bytes(1 * memory.MiB)
		err = uplink.UploadWithExpiration(ctx, satellite, "testbucket", "test", data, time.Now().Add(1*time.Hour))
		require.NoError(t, err)

		// get expired pieces
		checkExpired := func(expireAt time.Time, count int) {
			expired, err := planet.StorageNodes[1].StorageOld.Store.GetExpiredBatchSkipV0(ctx, expireAt, pieces.DefaultExpirationOptions())
			require.NoError(t, err)
			require.Len(t, expired, count)
		}
		checkExpired(time.Now().Add(2*time.Hour), 1)

		// run collector again but pieces are not expired yet
		err = planet.StorageNodes[1].StorageOld.Collector.Collect(ctx, time.Now())
		require.NoError(t, err)
		checkExpired(time.Now().Add(2*time.Hour), 1)

		// run collector again but pieces are expired
		err = planet.StorageNodes[1].StorageOld.Collector.Collect(ctx, time.Now().Add(2*time.Hour))
		require.NoError(t, err)
		checkExpired(time.Now().Add(2*time.Hour), 0)
	})
}

func TestCollector_oldDB(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		StorageNodeCount: 4, SatelliteCount: 1, UplinkCount: 1, MultinodeCount: 0,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
				config.Collector.Interval = -1
				// disable flat file store
				config.Pieces.EnableFlatExpirationStore = false
			},
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 2, 4, 4),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// disableMigrationBackend disables the migration backend for
		// this test to ensure that pieces with TTL are not moved to the
		// hashstore, allowing the test to focus on the collection of
		// expired pieces in the old store. This is necessary because
		// the test is specifically designed to verify the functionality
		// of the old collector.
		disableMigrationBackend(ctx, planet)

		// check that the collector runs without error when there are no expired pieces
		err := planet.StorageNodes[1].StorageOld.Collector.Collect(ctx, time.Now())
		require.NoError(t, err)

		uplink := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		data := testrand.Bytes(1 * memory.MiB)
		err = uplink.UploadWithExpiration(ctx, satellite, "testbucket", "test", data, time.Now().Add(1*time.Hour))
		require.NoError(t, err)

		// get expired pieces
		checkExpired := func(expireAt time.Time, count int) {
			expired, err := planet.StorageNodes[1].DB.PieceExpirationDB().GetExpired(ctx, expireAt, pieces.DefaultExpirationOptions())
			require.NoError(t, err)
			require.Len(t, expired, count)
		}
		checkExpired(time.Now().Add(2*time.Hour), 1)

		// run collector again but pieces are not expired yet
		err = planet.StorageNodes[1].StorageOld.Collector.Collect(ctx, time.Now())
		require.NoError(t, err)
		checkExpired(time.Now().Add(2*time.Hour), 1)

		// run collector again but pieces are expired
		err = planet.StorageNodes[1].StorageOld.Collector.Collect(ctx, time.Now().Add(2*time.Hour))
		require.NoError(t, err)

		// check that pieces are deleted
		checkExpired(time.Now().Add(2*time.Hour), 0)
	})
}

func disableMigrationBackend(ctx context.Context, planet *testplanet.Planet) {
	for _, sn := range planet.StorageNodes {
		for _, sat := range planet.Satellites {
			sn.Storage2.MigratingBackend.UpdateState(ctx, sat.ID(), func(state *piecestore.MigrationState) {
				state.PassiveMigrate = false
				state.WriteToNew = false
				state.ReadNewFirst = false
				state.TTLToNew = false
			})
			sn.Storage2.MigrationChore.SetMigrate(sat.ID(), false, false)
		}
	}
}
