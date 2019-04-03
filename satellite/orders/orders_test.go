// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storj"
)

func TestSendingReceivingOrders(t *testing.T) {
	// test happy path
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Sender.Loop.Pause()
		}

		expectedData := make([]byte, 50*memory.KiB)
		_, err := rand.Read(expectedData)
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		sumBeforeSend := 0
		for _, storageNode := range planet.StorageNodes {
			infos, err := storageNode.DB.Orders().ListUnsent(ctx, 10)
			require.NoError(t, err)
			sumBeforeSend += len(infos)
		}
		require.NotZero(t, sumBeforeSend)

		sumUnsent := 0
		sumArchived := 0

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Sender.Loop.TriggerWait()

			infos, err := storageNode.DB.Orders().ListUnsent(ctx, 10)
			require.NoError(t, err)
			sumUnsent += len(infos)

			archivedInfos, err := storageNode.DB.Orders().ListArchived(ctx, sumBeforeSend)
			require.NoError(t, err)
			sumArchived += len(archivedInfos)
		}

		require.Zero(t, sumUnsent)
		require.Equal(t, sumBeforeSend, sumArchived)
	})
}

func TestUnableToSendOrders(t *testing.T) {
	// test sending when satellite is unavailable
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Sender.Loop.Pause()
		}

		expectedData := make([]byte, 50*memory.KiB)
		_, err := rand.Read(expectedData)
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		sumBeforeSend := 0
		for _, storageNode := range planet.StorageNodes {
			infos, err := storageNode.DB.Orders().ListUnsent(ctx, 10)
			require.NoError(t, err)
			sumBeforeSend += len(infos)
		}
		require.NotZero(t, sumBeforeSend)

		err = planet.StopPeer(planet.Satellites[0])
		require.NoError(t, err)

		sumUnsent := 0
		sumArchived := 0
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Sender.Loop.TriggerWait()

			infos, err := storageNode.DB.Orders().ListUnsent(ctx, 10)
			require.NoError(t, err)
			sumUnsent += len(infos)

			archivedInfos, err := storageNode.DB.Orders().ListArchived(ctx, sumBeforeSend)
			require.NoError(t, err)
			sumArchived += len(archivedInfos)
		}

		require.Zero(t, sumArchived)
		require.Equal(t, sumBeforeSend, sumUnsent)
	})
}

func TestUploadDownloadBandwidth(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		hourBeforeTest := time.Now().Add(-time.Hour)

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Sender.Loop.Pause()
		}

		expectedData := make([]byte, 50*memory.KiB)
		_, err := rand.Read(expectedData)
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, expectedData, data)

		var expectedBucketBandwidth int64
		expectedStorageBandwidth := make(map[storj.NodeID]int64)
		for _, storageNode := range planet.StorageNodes {
			infos, err := storageNode.DB.Orders().ListUnsent(ctx, 10)
			require.NoError(t, err)
			if len(infos) > 0 {
				for _, info := range infos {
					expectedBucketBandwidth += info.Order.Amount
					expectedStorageBandwidth[storageNode.ID()] += info.Order.Amount
				}
			}
		}

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Sender.Loop.TriggerWait()
		}

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		ordersDB := planet.Satellites[0].DB.Orders()
		bucketID := storj.JoinPaths(projects[0].ID.String(), "testbucket")

		bucketBandwidth, err := ordersDB.GetBucketBandwidth(ctx, []byte(bucketID), hourBeforeTest, time.Now())
		require.NoError(t, err)
		require.Equal(t, expectedBucketBandwidth, bucketBandwidth)

		for _, storageNode := range planet.StorageNodes {
			nodeBandwidth, err := ordersDB.GetStorageNodeBandwidth(ctx, storageNode.ID(), hourBeforeTest, time.Now())
			require.NoError(t, err)
			require.Equal(t, expectedStorageBandwidth[storageNode.ID()], nodeBandwidth)
		}
	})
}
