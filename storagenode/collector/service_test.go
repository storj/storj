// Copyright (C) 2019 Storj Labs, Inc.
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
)

func TestCollector(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(1, 1, 2, 2),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		for _, storageNode := range planet.StorageNodes {
			// stop collector, so we can run it manually
			storageNode.Collector.Loop.Pause()
			// stop order sender because we will stop satellite later
			storageNode.Storage2.Orders.Sender.Pause()
		}

		expectedData := testrand.Bytes(100 * memory.KiB)

		// upload some data to exactly 2 nodes that expires in 8 days
		err := planet.Uplinks[0].UploadWithExpiration(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData, time.Now().Add(8*24*time.Hour))
		require.NoError(t, err)

		// stop satellite to prevent audits
		require.NoError(t, planet.StopPeer(planet.Satellites[0]))

		collections := 0
		serialsPresent := 0

		// imagine we are 30 minutes in the future
		for _, storageNode := range planet.StorageNodes {
			blobCache := storageNode.Storage2.BlobsCache
			usedSerials := storageNode.UsedSerials

			// verify that we actually have some data on storage nodes
			used, _, err := blobCache.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			if used == 0 {
				// this storage node didn't get picked for storing data
				continue
			}

			// collect all the data
			err = storageNode.Collector.Collect(ctx, time.Now().Add(30*time.Minute))
			require.NoError(t, err)

			serialsPresent += usedSerials.Count()

			collections++
		}

		require.NotZero(t, collections)
		// ensure we haven't deleted used serials
		require.Equal(t, 2, serialsPresent)

		serialsPresent = 0

		// imagine we are 2 hours in the future
		for _, storageNode := range planet.StorageNodes {
			usedSerials := storageNode.UsedSerials

			// collect all the data
			err = storageNode.Collector.Collect(ctx, time.Now().Add(2*time.Hour))
			require.NoError(t, err)

			serialsPresent += usedSerials.Count()

			collections++
		}

		// ensure we have deleted used serials
		require.Equal(t, 0, serialsPresent)

		// imagine we are 10 days in the future
		for _, storageNode := range planet.StorageNodes {
			blobCache := storageNode.Storage2.BlobsCache
			usedSerials := storageNode.UsedSerials

			// collect all the data
			err = storageNode.Collector.Collect(ctx, time.Now().Add(10*24*time.Hour))
			require.NoError(t, err)

			// verify that we deleted everything
			used, _, err := blobCache.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(0), used)

			serialsPresent += usedSerials.Count()

			collections++
		}

		// ensure we have deleted used serials
		require.Equal(t, 0, serialsPresent)
	})
}
