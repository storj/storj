// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func getSettledBandwidth(ctx context.Context, accountingDB accounting.ProjectAccounting, projectID uuid.UUID, since time.Time) (int64, error) {
	total, err := accountingDB.GetProjectSettledBandwidthTotal(ctx, projectID, since.Add(-time.Hour))
	if err != nil {
		return 0, err
	}
	return total, nil
}

// TestRollupsWriteCacheBatchLimitReached makes sure bandwidth rollup values are not written to the
// db until the batch size is reached.
func TestRollupsWriteCacheBatchLimitReached(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		useBatchSize := 10
		amount := (memory.MB * 500).Int64()
		projectID := testrand.UUID()
		startTime := time.Now()

		rwc := orders.NewRollupsWriteCache(zaptest.NewLogger(t), db.Orders(), useBatchSize)

		accountingDB := db.ProjectAccounting()

		expectedTotal := int64(0)
		// use different bucketName for each write, so they don't get aggregated yet
		for i := 0; i < useBatchSize-1; i++ {
			bucketName := fmt.Sprintf("my_files_%d", i)
			err := rwc.UpdateBucketBandwidthSettle(ctx, projectID, []byte(bucketName), pb.PieceAction_GET, amount, 0, startTime)
			require.NoError(t, err)

			// check that nothing was actually written since it should just be stored
			total, err := getSettledBandwidth(ctx, accountingDB, projectID, startTime)
			require.NoError(t, err)
			require.Zero(t, total)

			expectedTotal += amount
		}

		whenDone := rwc.OnNextFlush()
		// write one more rollup record to hit the threshold
		err := rwc.UpdateBucketBandwidthAllocation(ctx, projectID, []byte("my_files_last"), pb.PieceAction_GET, amount, startTime)
		require.NoError(t, err)

		// make sure flushing is done
		select {
		case <-whenDone:
			break
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}

		total, err := getSettledBandwidth(ctx, accountingDB, projectID, startTime)
		require.NoError(t, err)
		require.Equal(t, expectedTotal, total)
	})
}

// TestRollupsWriteCacheBatchChore makes sure bandwidth rollup values are not written to the
// db until the chore flushes the DB (assuming the batch size is not reached).
func TestRollupsWriteCacheBatchChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			useBatchSize := 10
			amount := (memory.MB * 500).Int64()
			projectID := testrand.UUID()
			startTime := time.Now()

			planet.Satellites[0].Orders.Chore.Loop.Pause()

			accountingDB := planet.Satellites[0].DB.ProjectAccounting()
			ordersDB := planet.Satellites[0].Orders.DB

			expectedTotal := int64(0)
			for i := 0; i < useBatchSize-1; i++ {
				bucketName := fmt.Sprintf("my_files_%d", i)
				err := ordersDB.UpdateBucketBandwidthSettle(ctx, projectID, []byte(bucketName), pb.PieceAction_GET, amount, 0, startTime)
				require.NoError(t, err)

				// check that nothing was actually written
				total, err := getSettledBandwidth(ctx, accountingDB, projectID, startTime)
				require.NoError(t, err)
				require.Zero(t, total)

				expectedTotal += amount
			}

			rwc := ordersDB.(*orders.RollupsWriteCache)
			whenDone := rwc.OnNextFlush()
			// wait for Loop to complete
			planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

			// make sure flushing is done
			select {
			case <-whenDone:
				break
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			total, err := getSettledBandwidth(ctx, accountingDB, projectID, startTime)
			require.NoError(t, err)
			require.Equal(t, expectedTotal, total)
		},
	)
}

func TestUpdateBucketBandwidth(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			// don't let the loop flush our cache while we're checking it
			planet.Satellites[0].Orders.Chore.Loop.Pause()
			ordersDB := planet.Satellites[0].Orders.DB

			// setup: check there is nothing in the cache to start
			cache, ok := ordersDB.(*orders.RollupsWriteCache)
			require.True(t, ok)
			size := cache.CurrentSize()
			require.Equal(t, 0, size)

			// setup: add an allocated and settled item to the cache
			projectID := testrand.UUID()
			bucketName := []byte("testbucketname")
			amount := (memory.MB * 500).Int64()
			intervalStart := time.Now()
			err := ordersDB.UpdateBucketBandwidthInline(ctx, projectID, bucketName, pb.PieceAction_GET, amount, intervalStart)
			require.NoError(t, err)
			err = ordersDB.UpdateBucketBandwidthSettle(ctx, projectID, bucketName, pb.PieceAction_PUT, amount, 0, intervalStart)
			require.NoError(t, err)

			// test: confirm there is one item in the cache now
			size = cache.CurrentSize()
			require.Equal(t, 2, size)
			projectMap := cache.CurrentData()
			expectedKeyAllocated := orders.CacheKey{
				ProjectID:     projectID,
				BucketName:    string(bucketName),
				Action:        pb.PieceAction_GET,
				IntervalStart: time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), intervalStart.Hour(), 0, 0, 0, intervalStart.Location()).Unix(),
			}
			expectedKeySettled := orders.CacheKey{
				ProjectID:     projectID,
				BucketName:    string(bucketName),
				Action:        pb.PieceAction_PUT,
				IntervalStart: time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), intervalStart.Hour(), 0, 0, 0, intervalStart.Location()).Unix(),
			}
			expectedCacheDataAllocated := orders.CacheData{
				Inline:    amount,
				Allocated: 0,
				Settled:   0,
			}
			expectedCacheDataSettled := orders.CacheData{
				Inline:    0,
				Allocated: 0,
				Settled:   amount,
			}
			require.Equal(t, expectedCacheDataAllocated, projectMap[expectedKeyAllocated])
			require.Equal(t, expectedCacheDataSettled, projectMap[expectedKeySettled])

			// setup: add another item to the cache but with a different projectID
			projectID2 := testrand.UUID()
			amount2 := (memory.MB * 10).Int64()
			err = ordersDB.UpdateBucketBandwidthInline(ctx, projectID2, bucketName, pb.PieceAction_GET, amount2, intervalStart)
			require.NoError(t, err)
			err = ordersDB.UpdateBucketBandwidthSettle(ctx, projectID2, bucketName, pb.PieceAction_GET, amount2, 0, intervalStart)
			require.NoError(t, err)
			size = cache.CurrentSize()
			require.Equal(t, 3, size)
			projectMap2 := cache.CurrentData()

			// test: confirm there are 3 items in the cache now with different projectIDs
			expectedKey := orders.CacheKey{
				ProjectID:     projectID2,
				BucketName:    string(bucketName),
				Action:        pb.PieceAction_GET,
				IntervalStart: time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), intervalStart.Hour(), 0, 0, 0, intervalStart.Location()).Unix(),
			}
			expectedData := orders.CacheData{
				Inline:    amount2,
				Allocated: 0,
				Settled:   amount2,
			}
			require.Equal(t, projectMap2[expectedKey], expectedData)
			require.Equal(t, len(projectMap2), 3)
		},
	)
}

func TestEndpointAndCacheContextCanceled(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Orders.FlushBatchSize = 3
			},
		},
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			storagenode := planet.StorageNodes[0]
			ordersDB := planet.Satellites[0].Orders.DB

			now := time.Now()

			// create orders to trigger RollupsWriteCache flush
			projectID := testrand.UUID()
			requests := []*pb.SettlementRequest{}
			singleOrderAmount := int64(50)
			for i := 0; i < 3; i++ {
				piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
				require.NoError(t, err)

				bucketname := metabase.BucketName("testbucket" + strconv.Itoa(i))

				bucketLocation := metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: bucketname,
				}

				serialNumber := testrand.SerialNumber()
				key := satellite.Config.Orders.EncryptionKeys.Default
				encrypted, err := key.EncryptMetadata(
					serialNumber,
					&internalpb.OrderLimitMetadata{
						CompactProjectBucketPrefix: bucketLocation.CompactPrefix(),
					},
				)
				require.NoError(t, err)

				limit := &pb.OrderLimit{
					SerialNumber:           serialNumber,
					SatelliteId:            satellite.ID(),
					UplinkPublicKey:        piecePublicKey,
					StorageNodeId:          storagenode.ID(),
					PieceId:                storj.NewPieceID(),
					Action:                 pb.PieceAction_GET,
					Limit:                  1000,
					PieceExpiration:        time.Time{},
					OrderCreation:          now,
					OrderExpiration:        now.Add(24 * time.Hour),
					EncryptedMetadataKeyId: key.ID[:],
					EncryptedMetadata:      encrypted,
				}

				orderLimit, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satellite.Identity), limit)
				require.NoError(t, err)

				order, err := signing.SignUplinkOrder(ctx, piecePrivateKey, &pb.Order{
					SerialNumber: serialNumber,
					Amount:       singleOrderAmount,
				})
				require.NoError(t, err)

				requests = append(requests, &pb.SettlementRequest{
					Limit: orderLimit,
					Order: order,
				})
			}

			conn, err := storagenode.Dialer.DialNodeURL(ctx, storj.NodeURL{ID: satellite.ID(), Address: satellite.Addr()})
			require.NoError(t, err)
			defer ctx.Check(conn.Close)

			stream, err := pb.NewDRPCOrdersClient(conn).SettlementWithWindow(ctx)
			require.NoError(t, err)
			defer ctx.Check(stream.Close)

			for _, request := range requests {
				err := stream.Send(&pb.SettlementRequest{
					Limit: request.Limit,
					Order: request.Order,
				})
				require.NoError(t, err)
			}
			require.NoError(t, err)
			resp, err := stream.CloseAndRecv()
			require.NoError(t, err)
			require.Equal(t, pb.SettlementWithWindowResponse_ACCEPTED, resp.Status)

			rwc := ordersDB.(*orders.RollupsWriteCache)
			whenDone := rwc.OnNextFlush()

			// make sure flushing is done
			select {
			case <-whenDone:
				break
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			// verify that orders were stored in DB
			bucketBandwidth, err := getSettledBandwidth(ctx, planet.Satellites[0].DB.ProjectAccounting(), projectID, now)
			require.NoError(t, err)
			require.Equal(t, singleOrderAmount*int64(len(requests)), bucketBandwidth)
		},
	)
}
