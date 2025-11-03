// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestSendingReceivingOrders(t *testing.T) {
	// test happy path
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		now := time.Now()
		tomorrow := now.Add(24 * time.Hour)

		planet.Satellites[0].Audit.Worker.Loop.Pause()
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.Sender.Pause()
		}

		expectedData := testrand.Bytes(50 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		// Wait for storage nodes to propagate all information.
		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		sumBeforeSend := 0
		for _, storageNode := range planet.StorageNodes {
			// change settle buffer so orders can be sent
			unsentMap, err := storageNode.OrdersStore.ListUnsentBySatellite(ctx, tomorrow)
			require.NoError(t, err)
			for _, satUnsent := range unsentMap {
				sumBeforeSend += len(satUnsent.InfoList)
			}
		}
		require.NotZero(t, sumBeforeSend)

		sumUnsent := 0
		sumArchived := 0

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.SendOrders(ctx, tomorrow)

			unsentMap, err := storageNode.OrdersStore.ListUnsentBySatellite(ctx, tomorrow)
			require.NoError(t, err)
			for _, satUnsent := range unsentMap {
				sumUnsent += len(satUnsent.InfoList)
			}

			archivedInfos, err := storageNode.OrdersStore.ListArchived()
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
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		now := time.Now()
		tomorrow := now.Add(24 * time.Hour)

		planet.Satellites[0].Audit.Worker.Loop.Pause()
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.Sender.Pause()
		}

		expectedData := testrand.Bytes(50 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		// Wait for storage nodes to propagate all information.
		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		sumBeforeSend := 0
		for _, storageNode := range planet.StorageNodes {
			unsentMap, err := storageNode.OrdersStore.ListUnsentBySatellite(ctx, tomorrow)
			require.NoError(t, err)
			for _, satUnsent := range unsentMap {
				sumBeforeSend += len(satUnsent.InfoList)
			}
		}
		require.NotZero(t, sumBeforeSend)

		err = planet.StopPeer(planet.Satellites[0])
		require.NoError(t, err)

		sumUnsent := 0
		sumArchived := 0
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.SendOrders(ctx, tomorrow)

			unsentMap, err := storageNode.OrdersStore.ListUnsentBySatellite(ctx, tomorrow)
			require.NoError(t, err)
			for _, satUnsent := range unsentMap {
				sumUnsent += len(satUnsent.InfoList)
			}

			archivedInfos, err := storageNode.OrdersStore.ListArchived()
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
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(4, 4, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		now := time.Now()
		tomorrow := now.Add(24 * time.Hour)
		beforeRollup := now.Add(-time.Hour - time.Second)
		afterRollup := now.Add(time.Hour + time.Second)
		bucketName := "testbucket"

		planet.Satellites[0].Audit.Worker.Loop.Pause()

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.Sender.Pause()
		}

		expectedData := testrand.Bytes(50 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "test/path", expectedData)
		require.NoError(t, err)

		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], bucketName, "test/path")
		require.NoError(t, err)
		require.Equal(t, expectedData, data)

		// Wait for the download to end and so the orders will be saved
		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		var expectedBucketBandwidth int64
		expectedStorageBandwidth := make(map[storj.NodeID]int64)
		for _, storageNode := range planet.StorageNodes {
			infos, err := storageNode.OrdersStore.ListUnsentBySatellite(ctx, tomorrow)
			require.NoError(t, err)
			for _, unsentInfo := range infos {
				for _, orderInfo := range unsentInfo.InfoList {
					expectedBucketBandwidth += orderInfo.Order.Amount
					expectedStorageBandwidth[storageNode.ID()] += orderInfo.Order.Amount
				}
			}
		}

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.SendOrders(ctx, tomorrow)
		}
		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

		ordersDB := planet.Satellites[0].DB.Orders()

		_, _, bucketBandwidth, err := ordersDB.TestGetBucketBandwidth(ctx, planet.Uplinks[0].Projects[0].ID, []byte(bucketName), beforeRollup, afterRollup)
		require.NoError(t, err)
		assert.Equal(t, expectedBucketBandwidth, bucketBandwidth)

		for _, storageNode := range planet.StorageNodes {
			nodeBandwidth, err := ordersDB.GetStorageNodeBandwidth(ctx, storageNode.ID(), beforeRollup, afterRollup)
			assert.NoError(t, err)
			assert.Equal(t, expectedStorageBandwidth[storageNode.ID()], nodeBandwidth)
		}
	})
}

func TestMultiProjectUploadDownloadBandwidth(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// stop any async flushes because we want to be sure when some values are
		// written to avoid races
		planet.Satellites[0].Orders.Chore.Loop.Pause()

		now := time.Now()
		tomorrow := now.Add(24 * time.Hour)
		beforeRollup := now.Add(-time.Hour - time.Second)
		afterRollup := now.Add(time.Hour + time.Second)

		planet.Satellites[0].Audit.Worker.Loop.Pause()

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.Sender.Pause()
		}

		// Upload some data to two different projects in different buckets.
		firstExpectedData := testrand.Bytes(50 * memory.KiB)
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket0", "test/path", firstExpectedData)
		require.NoError(t, err)
		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket0", "test/path")
		require.NoError(t, err)
		require.Equal(t, firstExpectedData, data)

		secondExpectedData := testrand.Bytes(100 * memory.KiB)
		err = planet.Uplinks[1].Upload(ctx, planet.Satellites[0], "testbucket1", "test/path", secondExpectedData)
		require.NoError(t, err)
		data, err = planet.Uplinks[1].Download(ctx, planet.Satellites[0], "testbucket1", "test/path")
		require.NoError(t, err)
		require.Equal(t, secondExpectedData, data)

		// Wait for storage nodes to propagate all information.
		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		// Have the nodes send up the orders.
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.SendOrders(ctx, tomorrow)
		}
		// flush rollups write cache
		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

		// Query and ensure that there's no data recorded for the bucket from the other project
		ordersDB := planet.Satellites[0].DB.Orders()
		uplink0Project := planet.Uplinks[0].Projects[0].ID
		uplink1Project := planet.Uplinks[1].Projects[0].ID

		_, _, wrongBucketBandwidth, err := ordersDB.TestGetBucketBandwidth(ctx, uplink0Project, []byte("testbucket1"), beforeRollup, afterRollup)
		require.NoError(t, err)
		require.Equal(t, int64(0), wrongBucketBandwidth)
		_, _, rightBucketBandwidth, err := ordersDB.TestGetBucketBandwidth(ctx, uplink0Project, []byte("testbucket0"), beforeRollup, afterRollup)
		require.NoError(t, err)
		require.Greater(t, rightBucketBandwidth, int64(0))

		_, _, wrongBucketBandwidth, err = ordersDB.TestGetBucketBandwidth(ctx, uplink1Project, []byte("testbucket0"), beforeRollup, afterRollup)
		require.NoError(t, err)
		require.Equal(t, int64(0), wrongBucketBandwidth)
		_, _, rightBucketBandwidth, err = ordersDB.TestGetBucketBandwidth(ctx, uplink1Project, []byte("testbucket1"), beforeRollup, afterRollup)
		require.NoError(t, err)
		require.Greater(t, rightBucketBandwidth, int64(0))
	})
}

func TestUpdateStoragenodeBandwidthSettleWithWindow(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		ordersDB := db.Orders()
		now := time.Now().UTC()
		projectID := testrand.UUID()
		bucketname := "testbucket"
		snID := storj.NodeID{1}
		windowTime := now.AddDate(0, 0, -1)
		actionAmounts := map[int32]int64{
			int32(pb.PieceAction_GET):    100,
			int32(pb.PieceAction_PUT):    200,
			int32(pb.PieceAction_DELETE): 300,
		}

		// confirm there aren't any records in the storagenodebandwidth or bucketbandwidth table
		// at the beginning of the test
		storagenodeID := storj.NodeID{1}
		snbw, err := ordersDB.GetStorageNodeBandwidth(ctx, storagenodeID, time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, int64(0), snbw)
		_, _, bucketbw, err := ordersDB.TestGetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, int64(0), bucketbw)

		// test: process an order from a storagenode that has not been processed before
		status, alreadyProcesed, err := ordersDB.UpdateStoragenodeBandwidthSettleWithWindow(ctx, snID, actionAmounts, windowTime)
		require.NoError(t, err)
		require.Equal(t, pb.SettlementWithWindowResponse_ACCEPTED, status)
		require.Equal(t, false, alreadyProcesed)
		// confirm a record for storagenode bandwidth has been created
		snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenodeID, time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, int64(600), snbw)

		// test: process an order from a storagenode that has already been processed
		// and the orders match the orders that were already processed
		status, alreadyProcesed, err = ordersDB.UpdateStoragenodeBandwidthSettleWithWindow(ctx, snID, actionAmounts, windowTime)
		require.NoError(t, err)
		require.Equal(t, pb.SettlementWithWindowResponse_ACCEPTED, status)
		require.Equal(t, true, alreadyProcesed)
		// confirm that no more records were created for storagenode bandwidth
		snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenodeID, time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, int64(600), snbw)

		// test: process an order from a storagenode that has already been processed
		// and the orders DO NOT match the orders that were already processed
		actionAmounts2 := map[int32]int64{
			int32(pb.PieceAction_GET):    50,
			int32(pb.PieceAction_PUT):    25,
			int32(pb.PieceAction_DELETE): 100,
		}
		status, alreadyProcesed, err = ordersDB.UpdateStoragenodeBandwidthSettleWithWindow(ctx, snID, actionAmounts2, windowTime)
		require.NoError(t, err)
		require.Equal(t, pb.SettlementWithWindowResponse_REJECTED, status)
		require.Equal(t, false, alreadyProcesed)
		// confirm that no more records were created for storagenode bandwidth
		snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenodeID, time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, int64(600), snbw)
	})
}

func TestSettledAmountsMatch(t *testing.T) {
	testCases := []struct {
		name               string
		rows               []*dbx.StoragenodeBandwidthRollup
		orderActionAmounts map[int32]int64
		expected           bool
	}{
		{"zero value", []*dbx.StoragenodeBandwidthRollup{}, map[int32]int64{}, true},
		{"nil value", nil, nil, false},
		{"more rows amount", []*dbx.StoragenodeBandwidthRollup{{Action: uint(pb.PieceAction_PUT), Settled: 100}, {Action: uint(pb.PieceAction_GET), Settled: 200}}, map[int32]int64{1: 200}, false},
		{"equal", []*dbx.StoragenodeBandwidthRollup{{Action: uint(pb.PieceAction_PUT), Settled: 100}, {Action: uint(pb.PieceAction_GET), Settled: 200}}, map[int32]int64{1: 100, 2: 200}, true},
		{"more orders amount", []*dbx.StoragenodeBandwidthRollup{{Action: uint(pb.PieceAction_PUT), Settled: 100}}, map[int32]int64{1: 200, 0: 100}, false},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			matches := satellitedb.SettledAmountsMatch(tt.rows, tt.orderActionAmounts)
			require.Equal(t, tt.expected, matches)
		})
	}
}

func TestProjectBandwidthDailyRollups(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 1,

		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(3, 3, 3, 3),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// stop any async flushes because we want to be sure when some values are
		// written to avoid races
		planet.Satellites[0].Orders.Chore.Loop.Pause()

		now := time.Now()
		tomorrow := now.Add(24 * time.Hour)

		planet.Satellites[0].Audit.Worker.Loop.Pause()

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.Sender.Pause()
		}

		expectedData := testrand.Bytes(50 * memory.KiB)
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket0", "test/path", expectedData)
		require.NoError(t, err)
		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket0", "test/path")
		require.NoError(t, err)
		require.Equal(t, expectedData, data)

		// Wait for storage nodes to propagate all information.
		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		// Have the nodes send up the orders.
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.SendOrders(ctx, tomorrow)
		}
		// flush rollups write cache
		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

		projectAccountingDB := planet.Satellites[0].DB.ProjectAccounting()

		year, month, day := now.Year(), now.Month(), now.Day()
		allocated, settled, dead, err := projectAccountingDB.GetProjectDailyBandwidth(ctx, planet.Uplinks[0].Projects[0].ID, year, month, day)
		require.NoError(t, err)
		assert.NotZero(t, allocated)
		assert.Equal(t, allocated, settled)
		assert.Zero(t, dead)
	})
}
