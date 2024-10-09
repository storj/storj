// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package piecetracker_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestObserverPieceTracker(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.PieceTracker.UpdateBatchSize = 3
				config.RangedLoop.Parallelism = 4
				config.RangedLoop.BatchSize = 4

				// configure RS
				config.Metainfo.RS.Min = 2
				config.Metainfo.RS.Repair = 3
				config.Metainfo.RS.Success = 4
				config.Metainfo.RS.Total = 4
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// ensure that the piece counts are empty
		pieceCounts, err := planet.Satellites[0].Overlay.DB.ActiveNodesPieceCounts(ctx)
		require.NoError(t, err)
		require.Equal(t, 4, len(pieceCounts))

		// Setup: create 50KiB of data for the uplink to upload
		testdata := testrand.Bytes(50 * memory.KiB)

		testBucket := "testbucket"
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], testBucket, "test/path", testdata)
		require.NoError(t, err)

		// Run the ranged loop
		_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		// Check that the piece counts are correct
		pieceCounts, err = planet.Satellites[0].Overlay.DB.ActiveNodesPieceCounts(ctx)
		require.NoError(t, err)
		require.True(t, len(pieceCounts) > 0)

		for node, count := range pieceCounts {
			require.Equal(t, int64(1), count, "node %s should have 1 piece", node)
		}

		// upload more objects
		numOfObjects := 10
		for i := 0; i < numOfObjects; i++ {
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], testBucket, fmt.Sprintf("test/path%d", i), testdata)
			require.NoError(t, err)
		}

		// Run the ranged loop again
		_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		// Check that the piece counts are correct
		pieceCounts, err = planet.Satellites[0].Overlay.DB.ActiveNodesPieceCounts(ctx)
		require.NoError(t, err)
		require.True(t, len(pieceCounts) > 0)

		for node, count := range pieceCounts {
			require.Equal(t, int64(numOfObjects+1), count, "node %s should have %d pieces", node, numOfObjects+1)
		}
	})
}
