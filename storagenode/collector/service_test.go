// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package collector_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/collector"
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
			pieceStore := storageNode.DB.Pieces()
			usedSerials := storageNode.UsedSerials

			// verify that we actually have some data on storage nodes
			used, err := pieceStore.SpaceUsedForBlobs(ctx)
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
			pieceStore := storageNode.DB.Pieces()
			usedSerials := storageNode.UsedSerials

			// collect all the data
			err = storageNode.Collector.Collect(ctx, time.Now().Add(10*24*time.Hour))
			require.NoError(t, err)

			// verify that we deleted everything
			used, err := pieceStore.SpaceUsedForBlobs(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(0), used)

			serialsPresent += usedSerials.Count()

			collections++
		}

		// ensure we have deleted used serials
		require.Equal(t, 0, serialsPresent)
	})
}

func TestCollector_fileNotFound(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(1, 1, 2, 2),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		for _, storageNode := range planet.StorageNodes {
			// stop collector, so we can start a new service manually
			storageNode.Collector.Loop.Stop()
			// stop order sender because we will stop satellite later
			storageNode.Storage2.Orders.Sender.Pause()
		}

		expectedData := testrand.Bytes(5 * memory.KiB)

		// upload some data to exactly 2 nodes that expires in 1 day
		err := planet.Uplinks[0].UploadWithExpiration(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData, time.Now().Add(1*24*time.Hour))
		require.NoError(t, err)

		// stop satellite to prevent audits
		require.NoError(t, planet.StopPeer(planet.Satellites[0]))

		collections := 0

		// assume we are 2 days in the future
		for _, storageNode := range planet.StorageNodes {
			pieceStore := storageNode.DB.Pieces()

			// verify that we actually have some data on storage nodes
			used, err := pieceStore.SpaceUsedForBlobs(ctx)
			require.NoError(t, err)
			if used == 0 {
				// this storage node didn't get picked for storing data
				continue
			}

			// delete file before collector service runs
			err = pieceStore.DeleteNamespace(ctx, planet.Satellites[0].Identity.ID.Bytes())
			require.NoError(t, err)

			// create new observed logger
			observedZapCore, observedLogs := observer.New(zap.InfoLevel)
			observedLogger := zap.New(observedZapCore)
			// start new collector service
			collectorService := collector.NewService(observedLogger, storageNode.Storage2.Store, storageNode.UsedSerials, storageNode.Config.Collector)
			// collect all the data
			err = collectorService.Collect(ctx, time.Now().Add(2*24*time.Hour))
			require.NoError(t, err)
			require.Equal(t, 2, observedLogs.Len())
			// check "file does not exist" log
			require.Equal(t, observedLogs.All()[0].Level, zapcore.WarnLevel)
			require.Equal(t, observedLogs.All()[0].Message, "file does not exist")
			// check piece info deleted from db log
			require.Equal(t, observedLogs.All()[1].Level, zapcore.InfoLevel)
			require.Equal(t, observedLogs.All()[1].Message, "deleted expired piece info from DB")

			collections++
		}

		require.NotZero(t, collections)
	})
}
