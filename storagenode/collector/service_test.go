// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package collector_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
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
