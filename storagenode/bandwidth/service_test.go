// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
)

// Simple test for ensuring the service Persists the cache to the DB.
func TestBandwidthService(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		testID1 := testrand.NodeID()

		for _, storageNode := range planet.StorageNodes {
			// stop bandwidth service, so we can run it manually
			storageNode.Bandwidth.Service.Loop.Pause()
			bandwidthCache := storageNode.Bandwidth.Cache

			err := bandwidthCache.Add(ctx, testID1, pb.PieceAction_PUT, 2, time.Now().Add(time.Hour*-2))
			require.NoError(t, err)
			err = bandwidthCache.Add(ctx, testID1, pb.PieceAction_GET, 3, time.Now().Add(time.Hour*-2))
			require.NoError(t, err)
			err = bandwidthCache.Add(ctx, testID1, pb.PieceAction_GET_AUDIT, 4, time.Now().Add(time.Hour*-2))
			require.NoError(t, err)

			// check that the bandwidth cache has the expected values
			usage, err := bandwidthCache.Summary(ctx, time.Time{}, time.Now())
			require.NoError(t, err)
			require.Equal(t, int64(9), usage.Total())

			// bandwidthdb should be empty
			usage, err = storageNode.DB.Bandwidth().Summary(ctx, time.Time{}, time.Now())
			require.NoError(t, err)
			require.Equal(t, int64(0), usage.Total())

			// run the bandwidth service
			storageNode.Bandwidth.Service.Loop.TriggerWait()

			// check that the bandwidth cache has been persisted to the db
			usage, err = storageNode.DB.Bandwidth().Summary(ctx, time.Time{}, time.Now())
			require.NoError(t, err)
			require.Equal(t, int64(9), usage.Total())

			// although the cache is cleared, the we should be able to get data through the cache
			// because it queries the db if the data is not in the cache
			usage, err = bandwidthCache.Summary(ctx, time.Time{}, time.Now())
			require.NoError(t, err)
			require.Equal(t, int64(9), usage.Total())
		}
	})
}
