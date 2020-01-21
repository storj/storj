// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/orders"
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
		planet.Satellites[0].Audit.Worker.Loop.Pause()
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.Sender.Pause()
		}

		expectedData := testrand.Bytes(50 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
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
			storageNode.Storage2.Orders.Sender.TriggerWait()

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
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Audit.Worker.Loop.Pause()
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.Sender.Pause()
		}

		expectedData := testrand.Bytes(50 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
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
			storageNode.Storage2.Orders.Sender.TriggerWait()

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
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		wayInTheFuture := time.Now().UTC().Add(1000 * time.Hour)
		hourBeforeTheFuture := wayInTheFuture.Add(-time.Hour)
		planet.Satellites[0].Audit.Worker.Loop.Pause()

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.Sender.Pause()
		}

		expectedData := testrand.Bytes(50 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, expectedData, data)

		//HACKFIX: We need enough time to pass after the download ends for storagenodes to save orders
		time.Sleep(200 * time.Millisecond)

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
			storageNode.Storage2.Orders.Sender.TriggerWait()
		}

		// Run the chore as if we were far in the future so that the orders are expired.
		reportedRollupChore := planet.Satellites[0].Core.Accounting.ReportedRollupChore
		require.NoError(t, reportedRollupChore.RunOnce(ctx, wayInTheFuture))

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		ordersDB := planet.Satellites[0].DB.Orders()

		bucketBandwidth, err := ordersDB.GetBucketBandwidth(ctx, projects[0].ID, []byte("testbucket"), hourBeforeTheFuture, wayInTheFuture)
		require.NoError(t, err)
		require.Equal(t, expectedBucketBandwidth, bucketBandwidth)

		for _, storageNode := range planet.StorageNodes {
			nodeBandwidth, err := ordersDB.GetStorageNodeBandwidth(ctx, storageNode.ID(), hourBeforeTheFuture, wayInTheFuture)
			require.NoError(t, err)
			require.Equal(t, expectedStorageBandwidth[storageNode.ID()], nodeBandwidth)
		}
	})
}

func TestSplitBucketIDInvalid(t *testing.T) {
	var testCases = []struct {
		name     string
		bucketID []byte
	}{
		{"invalid, not valid UUID", []byte("not UUID string/bucket1")},
		{"invalid, not valid UUID, no bucket", []byte("not UUID string")},
		{"invalid, no project, no bucket", []byte("")},
	}
	for _, tt := range testCases {
		tt := tt // avoid scopelint error, ref: https://github.com/golangci/golangci-lint/issues/281
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := orders.SplitBucketID(tt.bucketID)
			assert.NotNil(t, err)
			assert.Error(t, err)
		})
	}
}

func TestSplitBucketIDValid(t *testing.T) {
	var testCases = []struct {
		name               string
		project            string
		bucketName         string
		expectedBucketName string
	}{
		{"valid, no bucket, no objects", "bb6218e3-4b4a-4819-abbb-fa68538e33c0", "", ""},
		{"valid, with bucket", "bb6218e3-4b4a-4819-abbb-fa68538e33c0", "testbucket", "testbucket"},
		{"valid, with object", "bb6218e3-4b4a-4819-abbb-fa68538e33c0", "testbucket/foo/bar.txt", "testbucket"},
	}
	for _, tt := range testCases {
		tt := tt // avoid scopelint error, ref: https://github.com/golangci/golangci-lint/issues/281
		t.Run(tt.name, func(t *testing.T) {
			expectedProjectID, err := uuid.Parse(tt.project)
			assert.NoError(t, err)
			bucketID := expectedProjectID.String() + "/" + tt.bucketName

			actualProjectID, actualBucketName, err := orders.SplitBucketID([]byte(bucketID))
			assert.NoError(t, err)
			assert.Equal(t, actualProjectID, expectedProjectID)
			assert.Equal(t, actualBucketName, []byte(tt.expectedBucketName))
		})
	}
}

func BenchmarkOrders(b *testing.B) {
	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	counts := []int{50, 100, 250, 500, 1000}
	for _, c := range counts {
		c := c
		satellitedbtest.Bench(b, func(b *testing.B, db satellite.DB) {
			snID := testrand.NodeID()

			projectID, _ := uuid.New()
			bucketID := []byte(projectID.String() + "/b")

			b.Run("Benchmark Order Processing:"+strconv.Itoa(c), func(b *testing.B) {
				ctx := context.Background()
				for i := 0; i < b.N; i++ {
					requests := buildBenchmarkData(ctx, b, db, snID, bucketID, c)

					_, err := db.Orders().ProcessOrders(ctx, requests)
					assert.NoError(b, err)
				}
			})
		})
	}

}

func buildBenchmarkData(ctx context.Context, b *testing.B, db satellite.DB, storageNodeID storj.NodeID, bucketID []byte, orderCount int) (_ []*orders.ProcessOrderRequest) {
	requests := make([]*orders.ProcessOrderRequest, 0, orderCount)

	for i := 0; i < orderCount; i++ {
		snUUID, _ := uuid.New()
		sn, err := storj.SerialNumberFromBytes(snUUID[:])
		require.NoError(b, err)

		err = db.Orders().CreateSerialInfo(ctx, sn, bucketID, time.Now().Add(time.Hour*24))
		require.NoError(b, err)

		order := &pb.Order{
			SerialNumber: sn,
			Amount:       1,
		}

		orderLimit := &pb.OrderLimit{
			SerialNumber:  sn,
			StorageNodeId: storageNodeID,
			Action:        2,
		}
		requests = append(requests, &orders.ProcessOrderRequest{Order: order,
			OrderLimit: orderLimit})
	}
	return requests
}

func TestProcessOrders(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		ordersDB := db.Orders()
		serialNum := storj.SerialNumber{1}
		serialNum2 := storj.SerialNumber{2}
		projectID, _ := uuid.New()

		// setup: create serial number records
		err := ordersDB.CreateSerialInfo(ctx, serialNum, []byte(projectID.String()+"/b"), time.Now().AddDate(0, 0, 1))
		require.NoError(t, err)
		err = ordersDB.CreateSerialInfo(ctx, serialNum2, []byte(projectID.String()+"/c"), time.Now().AddDate(0, 0, 1))
		require.NoError(t, err)

		requests := []*orders.ProcessOrderRequest{
			{
				Order: &pb.Order{
					SerialNumber: serialNum,
					Amount:       100,
				},
				OrderLimit: &pb.OrderLimit{
					SerialNumber:    serialNum,
					StorageNodeId:   storj.NodeID{1},
					Action:          pb.PieceAction_DELETE,
					OrderExpiration: time.Now().AddDate(0, 0, 3),
				},
			},
		}
		// test: process one order and confirm we get the correct response
		actualResponses, err := ordersDB.ProcessOrders(ctx, requests)
		require.NoError(t, err)
		expectedResponses := []*orders.ProcessOrderResponse{
			{
				SerialNumber: serialNum,
				Status:       pb.SettlementResponse_ACCEPTED,
			},
		}
		assert.Equal(t, expectedResponses, actualResponses)

		requests = append(requests, &orders.ProcessOrderRequest{
			Order: &pb.Order{
				SerialNumber: serialNum2,
				Amount:       200,
			},
			OrderLimit: &pb.OrderLimit{
				SerialNumber:    serialNum2,
				StorageNodeId:   storj.NodeID{2},
				Action:          pb.PieceAction_PUT,
				OrderExpiration: time.Now().AddDate(0, 0, 1)},
		})
		// test: process two orders from different storagenodes and confirm there is an error
		_, err = ordersDB.ProcessOrders(ctx, requests)
		require.Error(t, err, "different storage nodes")

		requests[0].OrderLimit.StorageNodeId = storj.NodeID{2}
		// test: process two orders from same storagenodes and confirm we get two responses
		actualResponses, err = ordersDB.ProcessOrders(ctx, requests)
		require.NoError(t, err)
		assert.Equal(t, 2, len(actualResponses))

		// test: confirm the correct data from processing orders was written to reported_serials table
		bbr, snr, err := ordersDB.GetBillableBandwidth(ctx, time.Now().AddDate(0, 0, 3))
		require.NoError(t, err)
		assert.Equal(t, 1, len(bbr))
		expected := []orders.BucketBandwidthRollup{
			{
				ProjectID:  *projectID,
				BucketName: "c",
				Action:     pb.PieceAction_PUT,
				Inline:     0,
				Allocated:  0,
				Settled:    200,
			},
		}
		assert.Equal(t, expected, bbr)
		assert.Equal(t, 1, len(snr))
		expectedRollup := []orders.StoragenodeBandwidthRollup{
			{
				NodeID:    storj.NodeID{2},
				Action:    pb.PieceAction_PUT,
				Allocated: 0,
				Settled:   200,
			},
		}
		assert.Equal(t, expectedRollup, snr)
		bbr, snr, err = ordersDB.GetBillableBandwidth(ctx, time.Now().AddDate(0, 0, 5))
		require.NoError(t, err)
		assert.Equal(t, 2, len(bbr))
		assert.Equal(t, 3, len(snr))
	})
}
