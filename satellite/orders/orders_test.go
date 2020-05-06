// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting/reportedrollup"
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
		now := time.Now()
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

		reportedRollupChore := planet.Satellites[0].Core.Accounting.ReportedRollupChore
		require.NoError(t, reportedRollupChore.RunOnce(ctx, now))

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		ordersDB := planet.Satellites[0].DB.Orders()

		bucketBandwidth, err := ordersDB.GetBucketBandwidth(ctx, projects[0].ID, []byte(bucketName), beforeRollup, afterRollup)
		require.NoError(t, err)
		require.Equal(t, expectedBucketBandwidth, bucketBandwidth)

		for _, storageNode := range planet.StorageNodes {
			nodeBandwidth, err := ordersDB.GetStorageNodeBandwidth(ctx, storageNode.ID(), beforeRollup, afterRollup)
			require.NoError(t, err)
			require.Equal(t, expectedStorageBandwidth[storageNode.ID()], nodeBandwidth)
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
		now := time.Now()
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

		//HACKFIX: We need enough time to pass after the download ends for storagenodes to save orders
		time.Sleep(200 * time.Millisecond)

		// Have the nodes send up the orders.
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.Sender.TriggerWait()
		}

		// Run the chore as if we were far in the future so that the orders are expired.
		reportedRollupChore := planet.Satellites[0].Core.Accounting.ReportedRollupChore
		require.NoError(t, reportedRollupChore.RunOnce(ctx, now))

		// Query and ensure that there's no data recorded for the bucket from the other project
		ordersDB := planet.Satellites[0].DB.Orders()
		uplink0Project := planet.Uplinks[0].Projects[0].ID
		uplink1Project := planet.Uplinks[1].Projects[0].ID

		wrongBucketBandwidth, err := ordersDB.GetBucketBandwidth(ctx, uplink0Project, []byte("testbucket1"), beforeRollup, afterRollup)
		require.NoError(t, err)
		require.Equal(t, int64(0), wrongBucketBandwidth)
		rightBucketBandwidth, err := ordersDB.GetBucketBandwidth(ctx, uplink0Project, []byte("testbucket0"), beforeRollup, afterRollup)
		require.NoError(t, err)
		require.Greater(t, rightBucketBandwidth, int64(0))

		wrongBucketBandwidth, err = ordersDB.GetBucketBandwidth(ctx, uplink1Project, []byte("testbucket0"), beforeRollup, afterRollup)
		require.NoError(t, err)
		require.Equal(t, int64(0), wrongBucketBandwidth)
		rightBucketBandwidth, err = ordersDB.GetBucketBandwidth(ctx, uplink1Project, []byte("testbucket1"), beforeRollup, afterRollup)
		require.NoError(t, err)
		require.Greater(t, rightBucketBandwidth, int64(0))
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
			expectedProjectID, err := uuid.FromString(tt.project)
			assert.NoError(t, err)
			bucketID := expectedProjectID.String() + "/" + tt.bucketName

			actualProjectID, actualBucketName, err := orders.SplitBucketID([]byte(bucketID))
			assert.NoError(t, err)
			assert.Equal(t, expectedProjectID, actualProjectID)
			assert.Equal(t, []byte(tt.expectedBucketName), actualBucketName)
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

func TestLargeOrderLimit(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		ordersDB := db.Orders()
		chore := reportedrollup.NewChore(zaptest.NewLogger(t), ordersDB, reportedrollup.Config{})
		serialNum := storj.SerialNumber{1}

		projectID, _ := uuid.New()
		now := time.Now()
		beforeRollup := now.Add(-time.Hour)
		afterRollup := now.Add(time.Hour)

		// setup: create serial number records
		err := ordersDB.CreateSerialInfo(ctx, serialNum, []byte(projectID.String()+"/b"), now.AddDate(0, 0, 1))
		require.NoError(t, err)

		var requests []*orders.ProcessOrderRequest

		// process one order with smaller amount than the order limit and confirm we get the correct response
		{
			requests = append(requests, &orders.ProcessOrderRequest{
				Order: &pb.Order{
					SerialNumber: serialNum,
					Amount:       100,
				},
				OrderLimit: &pb.OrderLimit{
					SerialNumber:    serialNum,
					StorageNodeId:   storj.NodeID{1},
					Action:          pb.PieceAction_GET,
					OrderExpiration: now.AddDate(0, 0, 3),
					Limit:           250,
				},
			})
			actualResponses, err := ordersDB.ProcessOrders(ctx, requests)
			require.NoError(t, err)
			expectedResponses := []*orders.ProcessOrderResponse{
				{
					SerialNumber: serialNum,
					Status:       pb.SettlementResponse_ACCEPTED,
				},
			}
			assert.Equal(t, expectedResponses, actualResponses)

			require.NoError(t, chore.RunOnce(ctx, now))

			// check only the bandwidth we've used is taken into account
			bucketBandwidth, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte("b"), beforeRollup, afterRollup)
			require.NoError(t, err)
			require.Equal(t, int64(100), bucketBandwidth)

			storageNodeBandwidth, err := ordersDB.GetStorageNodeBandwidth(ctx, storj.NodeID{1}, beforeRollup, afterRollup)
			require.NoError(t, err)
			require.Equal(t, int64(100), storageNodeBandwidth)
		}
	})
}

func TestProcessOrders(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		ordersDB := db.Orders()
		chore := reportedrollup.NewChore(zaptest.NewLogger(t), ordersDB, reportedrollup.Config{})
		invalidSerial := storj.SerialNumber{1}
		serialNum := storj.SerialNumber{2}
		serialNum2 := storj.SerialNumber{3}

		projectID, _ := uuid.New()
		now := time.Now()
		beforeRollup := now.Add(-time.Hour - time.Second)
		afterRollup := now.Add(time.Hour + time.Second)

		// assertion helpers
		checkBucketBandwidth := func(bucket string, amount int64) {
			settled, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucket), beforeRollup, afterRollup)
			require.NoError(t, err)
			require.Equal(t, amount, settled)
		}
		checkStoragenodeBandwidth := func(node storj.NodeID, amount int64) {
			settled, err := ordersDB.GetStorageNodeBandwidth(ctx, node, beforeRollup, afterRollup)
			require.NoError(t, err)
			require.Equal(t, amount, settled)
		}

		// setup: create serial number records
		err := ordersDB.CreateSerialInfo(ctx, serialNum, []byte(projectID.String()+"/b"), now.AddDate(0, 0, 1))
		require.NoError(t, err)
		err = ordersDB.CreateSerialInfo(ctx, serialNum2, []byte(projectID.String()+"/c"), now.AddDate(0, 0, 1))
		require.NoError(t, err)

		var requests []*orders.ProcessOrderRequest

		// process one order and confirm we get the correct response
		{
			requests = append(requests, &orders.ProcessOrderRequest{
				Order: &pb.Order{
					SerialNumber: serialNum,
					Amount:       100,
				},
				OrderLimit: &pb.OrderLimit{
					SerialNumber:    serialNum,
					StorageNodeId:   storj.NodeID{1},
					Action:          pb.PieceAction_DELETE,
					OrderExpiration: now.AddDate(0, 0, 3),
				},
			})
			actualResponses, err := ordersDB.ProcessOrders(ctx, requests)
			require.NoError(t, err)
			expectedResponses := []*orders.ProcessOrderResponse{
				{
					SerialNumber: serialNum,
					Status:       pb.SettlementResponse_ACCEPTED,
				},
			}
			assert.Equal(t, expectedResponses, actualResponses)
		}

		// process two orders from different storagenodes and confirm there is an error
		{
			requests = append(requests, &orders.ProcessOrderRequest{
				Order: &pb.Order{
					SerialNumber: serialNum2,
					Amount:       200,
				},
				OrderLimit: &pb.OrderLimit{
					SerialNumber:    serialNum2,
					StorageNodeId:   storj.NodeID{2},
					Action:          pb.PieceAction_PUT,
					OrderExpiration: now.AddDate(0, 0, 1)},
			})
			_, err = ordersDB.ProcessOrders(ctx, requests)
			require.Error(t, err, "different storage nodes")
		}

		// process two orders from same storagenodes and confirm we get two responses
		{
			requests[0].OrderLimit.StorageNodeId = storj.NodeID{2}
			actualResponses, err := ordersDB.ProcessOrders(ctx, requests)
			require.NoError(t, err)
			assert.Equal(t, 2, len(actualResponses))
		}

		// confirm the correct data from processing orders was written and consumed
		{
			require.NoError(t, chore.RunOnce(ctx, now))

			checkBucketBandwidth("b", 200)
			checkBucketBandwidth("c", 200)
			checkStoragenodeBandwidth(storj.NodeID{1}, 100)
			checkStoragenodeBandwidth(storj.NodeID{2}, 300)
		}

		// confirm invalid order at index 0 does not result in a SQL error
		{
			requests := []*orders.ProcessOrderRequest{
				{
					Order: &pb.Order{
						SerialNumber: invalidSerial,
						Amount:       200,
					},
					OrderLimit: &pb.OrderLimit{
						SerialNumber:    invalidSerial,
						StorageNodeId:   storj.NodeID{1},
						Action:          pb.PieceAction_PUT,
						OrderExpiration: now.AddDate(0, 0, 1),
					},
				},
				{
					Order: &pb.Order{
						SerialNumber: serialNum,
						Amount:       200,
					},
					OrderLimit: &pb.OrderLimit{
						SerialNumber:    serialNum,
						StorageNodeId:   storj.NodeID{1},
						Action:          pb.PieceAction_PUT,
						OrderExpiration: now.AddDate(0, 0, 1),
					},
				},
			}
			responses, err := ordersDB.ProcessOrders(ctx, requests)
			require.NoError(t, err)
			assert.Equal(t, pb.SettlementResponse_REJECTED, responses[0].Status)
		}

		// in case of conflicting ProcessOrderRequests, what has been recorded already wins
		{
			// unique nodeID so the other tests here don't interfere
			nodeID := testrand.NodeID()
			requests := []*orders.ProcessOrderRequest{
				{
					Order: &pb.Order{
						SerialNumber: serialNum,
						Amount:       100,
					},
					OrderLimit: &pb.OrderLimit{
						SerialNumber:    serialNum,
						StorageNodeId:   nodeID,
						Action:          pb.PieceAction_GET,
						OrderExpiration: now.AddDate(0, 0, 1),
					},
				},
				{
					Order: &pb.Order{
						SerialNumber: serialNum2,
						Amount:       200,
					},
					OrderLimit: &pb.OrderLimit{
						SerialNumber:    serialNum2,
						StorageNodeId:   nodeID,
						Action:          pb.PieceAction_GET,
						OrderExpiration: now.AddDate(0, 0, 1),
					},
				},
			}
			responses, err := ordersDB.ProcessOrders(ctx, requests)
			require.NoError(t, err)
			require.Equal(t, pb.SettlementResponse_ACCEPTED, responses[0].Status)
			require.Equal(t, pb.SettlementResponse_ACCEPTED, responses[1].Status)

			requests = []*orders.ProcessOrderRequest{
				{
					Order: &pb.Order{
						SerialNumber: serialNum,
						Amount:       1,
					},
					OrderLimit: &pb.OrderLimit{
						SerialNumber:    serialNum,
						StorageNodeId:   nodeID,
						Action:          pb.PieceAction_GET,
						OrderExpiration: now.AddDate(0, 0, 1),
					},
				},
				{
					Order: &pb.Order{
						SerialNumber: serialNum2,
						Amount:       500,
					},
					OrderLimit: &pb.OrderLimit{
						SerialNumber:    serialNum2,
						StorageNodeId:   nodeID,
						Action:          pb.PieceAction_GET,
						OrderExpiration: now.AddDate(0, 0, 1),
					},
				},
			}
			responses, err = ordersDB.ProcessOrders(ctx, requests)
			require.NoError(t, err)
			require.Equal(t, pb.SettlementResponse_ACCEPTED, responses[0].Status)
			require.Equal(t, pb.SettlementResponse_ACCEPTED, responses[1].Status)

			require.NoError(t, chore.RunOnce(ctx, now))

			checkBucketBandwidth("b", 201)
			checkBucketBandwidth("c", 700)
			checkStoragenodeBandwidth(storj.NodeID{1}, 100)
			checkStoragenodeBandwidth(storj.NodeID{2}, 300)
			checkStoragenodeBandwidth(nodeID, 501)
		}
	})
}

func TestRandomSampleLimits(t *testing.T) {
	orderlimits := []*pb.AddressedOrderLimit{{}, {}, {}, {}}

	s := orders.NewService(nil, nil, nil, nil, 0, nil, 0, false)
	t.Run("sample size is less than the number of order limits", func(t *testing.T) {
		var nilCount int
		sampleSize := 2
		limits, err := s.RandomSampleOfOrderLimits(orderlimits, sampleSize)
		assert.NoError(t, err)
		assert.Equal(t, len(orderlimits), len(limits))

		for _, limit := range limits {
			if limit == nil {
				nilCount++
			}
		}
		assert.Equal(t, len(orderlimits)-sampleSize, nilCount)
	})

	t.Run("sample size is greater than the number of order limits", func(t *testing.T) {
		var nilCount int
		sampleSize := 6
		limits, err := s.RandomSampleOfOrderLimits(orderlimits, sampleSize)
		assert.NoError(t, err)
		assert.Equal(t, len(orderlimits), len(limits))
		for _, limit := range limits {
			if limit == nil {
				nilCount++
			}
		}
		assert.Equal(t, 0, nilCount)
	})
}

func TestProcessOrders_DoubleSend(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		ordersDB := db.Orders()
		chore := reportedrollup.NewChore(zaptest.NewLogger(t), ordersDB, reportedrollup.Config{})
		serialNum := storj.SerialNumber{2}
		projectID, _ := uuid.New()
		now := time.Now()
		beforeRollup := now.Add(-time.Hour - time.Second)
		afterRollup := now.Add(time.Hour + time.Second)

		// assertion helpers
		checkBucketBandwidth := func(bucket string, amount int64) {
			settled, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucket), beforeRollup, afterRollup)
			require.NoError(t, err)
			require.Equal(t, amount, settled)
		}
		checkStoragenodeBandwidth := func(node storj.NodeID, amount int64) {
			settled, err := ordersDB.GetStorageNodeBandwidth(ctx, node, beforeRollup, afterRollup)
			require.NoError(t, err)
			require.Equal(t, amount, settled)
		}

		// setup: create serial number records
		err := ordersDB.CreateSerialInfo(ctx, serialNum, []byte(projectID.String()+"/b"), now.AddDate(0, 0, 1))
		require.NoError(t, err)

		order := &orders.ProcessOrderRequest{
			Order: &pb.Order{
				SerialNumber: serialNum,
				Amount:       100,
			},
			OrderLimit: &pb.OrderLimit{
				SerialNumber:    serialNum,
				StorageNodeId:   storj.NodeID{1},
				Action:          pb.PieceAction_PUT,
				OrderExpiration: now.AddDate(0, 0, 3),
			},
		}

		// send the same order twice in the same request
		{
			actualResponses, err := ordersDB.ProcessOrders(ctx, []*orders.ProcessOrderRequest{order, order})
			require.NoError(t, err)
			expectedResponses := []*orders.ProcessOrderResponse{
				{
					SerialNumber: serialNum,
					Status:       pb.SettlementResponse_ACCEPTED,
				},
				{
					SerialNumber: serialNum,
					Status:       pb.SettlementResponse_REJECTED,
				},
			}
			assert.Equal(t, expectedResponses, actualResponses)
		}

		// confirm the correct data from processing orders was written and consumed
		{
			require.NoError(t, chore.RunOnce(ctx, now))

			checkBucketBandwidth("b", 100)
			checkStoragenodeBandwidth(storj.NodeID{1}, 100)
		}

		// send the already sent and handled order again
		{
			actualResponses, err := ordersDB.ProcessOrders(ctx, []*orders.ProcessOrderRequest{order})
			require.NoError(t, err)
			expectedResponses := []*orders.ProcessOrderResponse{
				{
					SerialNumber: serialNum,
					Status:       pb.SettlementResponse_ACCEPTED,
				},
			}
			assert.Equal(t, expectedResponses, actualResponses)
		}

		// confirm the correct data from processing orders was written and consumed
		{
			require.NoError(t, chore.RunOnce(ctx, now))

			checkBucketBandwidth("b", 100)
			checkStoragenodeBandwidth(storj.NodeID{1}, 100)
		}
	})
}
