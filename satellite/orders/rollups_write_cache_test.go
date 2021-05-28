// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// unfortunately, in GB is apparently the only way we can get this data, so we need to
// arrange for the test to produce a total value that won't lose too much precision
// in the conversion to GB.
func getTotalBandwidthInGB(ctx context.Context, accountingDB accounting.ProjectAccounting, projectID uuid.UUID, since time.Time) (int64, error) {
	total, err := accountingDB.GetAllocatedBandwidthTotal(ctx, projectID, since.Add(-time.Hour))
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

		// use different bucketName for each write, so they don't get aggregated yet
		for i := 0; i < useBatchSize-1; i++ {
			bucketName := fmt.Sprintf("my_files_%d", i)
			err := rwc.UpdateBucketBandwidthAllocation(ctx, projectID, []byte(bucketName), pb.PieceAction_GET, amount, startTime)
			require.NoError(t, err)

			// check that nothing was actually written since it should just be stored
			total, err := getTotalBandwidthInGB(ctx, accountingDB, projectID, startTime)
			require.NoError(t, err)
			require.Equal(t, int64(0), total)
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

		total, err := getTotalBandwidthInGB(ctx, accountingDB, projectID, startTime)
		require.NoError(t, err)
		require.Equal(t, amount*int64(useBatchSize), total)
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

			// use different pieceAction for each write, so they don't get aggregated yet
			for i := 0; i < useBatchSize-1; i++ {
				bucketName := fmt.Sprintf("my_files_%d", i)
				err := ordersDB.UpdateBucketBandwidthAllocation(ctx, projectID, []byte(bucketName), pb.PieceAction_GET, amount, startTime)
				require.NoError(t, err)

				// check that nothing was actually written
				total, err := getTotalBandwidthInGB(ctx, accountingDB, projectID, startTime)
				require.NoError(t, err)
				require.Equal(t, int64(0), total)
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

			total, err := getTotalBandwidthInGB(ctx, accountingDB, projectID, startTime)
			require.NoError(t, err)
			require.Equal(t, amount*int64(useBatchSize-1), total)
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
			err := ordersDB.UpdateBucketBandwidthAllocation(ctx, projectID, bucketName, pb.PieceAction_GET, amount, time.Now())
			require.NoError(t, err)
			err = ordersDB.UpdateBucketBandwidthSettle(ctx, projectID, bucketName, pb.PieceAction_PUT, amount, time.Now())
			require.NoError(t, err)

			// test: confirm there is one item in the cache now
			size = cache.CurrentSize()
			require.Equal(t, 2, size)
			projectMap := cache.CurrentData()
			expectedKeyAllocated := orders.CacheKey{
				ProjectID:  projectID,
				BucketName: string(bucketName),
				Action:     pb.PieceAction_GET,
			}
			expectedKeySettled := orders.CacheKey{
				ProjectID:  projectID,
				BucketName: string(bucketName),
				Action:     pb.PieceAction_PUT,
			}
			expectedCacheDataAllocated := orders.CacheData{
				Inline:    0,
				Allocated: amount,
				Settled:   0,
			}
			expectedCacheDataSettled := orders.CacheData{
				Inline:    0,
				Allocated: 0,
				Settled:   amount,
			}
			require.Equal(t, projectMap[expectedKeyAllocated], expectedCacheDataAllocated)
			require.Equal(t, projectMap[expectedKeySettled], expectedCacheDataSettled)

			// setup: add another item to the cache but with a different projectID
			projectID2 := testrand.UUID()
			amount2 := (memory.MB * 10).Int64()
			err = ordersDB.UpdateBucketBandwidthAllocation(ctx, projectID2, bucketName, pb.PieceAction_GET, amount2, time.Now())
			require.NoError(t, err)
			err = ordersDB.UpdateBucketBandwidthSettle(ctx, projectID2, bucketName, pb.PieceAction_GET, amount2, time.Now())
			require.NoError(t, err)
			size = cache.CurrentSize()
			require.Equal(t, 3, size)
			projectMap2 := cache.CurrentData()

			// test: confirm there are 3 items in the cache now with different projectIDs
			expectedKey := orders.CacheKey{
				ProjectID:  projectID2,
				BucketName: string(bucketName),
				Action:     pb.PieceAction_GET,
			}
			expectedData := orders.CacheData{
				Inline:    0,
				Allocated: amount2,
				Settled:   amount2,
			}
			require.Equal(t, projectMap2[expectedKey], expectedData)
			require.Equal(t, len(projectMap2), 3)
		},
	)
}

func TestSortRollups(t *testing.T) {
	rollups := []orders.BucketBandwidthRollup{
		{
			ProjectID:  uuid.UUID{1},
			BucketName: "a",
			Action:     pb.PieceAction_GET, // GET is 2
			Inline:     1,
			Allocated:  2,
		},
		{
			ProjectID:  uuid.UUID{2},
			BucketName: "a",
			Action:     pb.PieceAction_GET,
			Inline:     1,
			Allocated:  2,
		},
		{
			ProjectID:  uuid.UUID{1},
			BucketName: "b",
			Action:     pb.PieceAction_GET,
			Inline:     1,
			Allocated:  2,
		},
		{
			ProjectID:  uuid.UUID{1},
			BucketName: "a",
			Action:     pb.PieceAction_GET_AUDIT,
			Inline:     1,
			Allocated:  2,
		},
		{
			ProjectID:  uuid.UUID{1},
			BucketName: "a",
			Action:     pb.PieceAction_GET,
			Inline:     1,
			Allocated:  2,
		},
	}

	expRollups := []orders.BucketBandwidthRollup{
		{
			ProjectID:  uuid.UUID{1},
			BucketName: "a",
			Action:     pb.PieceAction_GET, // GET is 2
			Inline:     1,
			Allocated:  2,
		},
		{
			ProjectID:  uuid.UUID{1},
			BucketName: "a",
			Action:     pb.PieceAction_GET,
			Inline:     1,
			Allocated:  2,
		},
		{
			ProjectID:  uuid.UUID{1},
			BucketName: "a",
			Action:     pb.PieceAction_GET_AUDIT,
			Inline:     1,
			Allocated:  2,
		},
		{
			ProjectID:  uuid.UUID{1},
			BucketName: "b",
			Action:     pb.PieceAction_GET,
			Inline:     1,
			Allocated:  2,
		},
		{
			ProjectID:  uuid.UUID{2},
			BucketName: "a",
			Action:     pb.PieceAction_GET,
			Inline:     1,
			Allocated:  2,
		},
	}

	assert.NotEqual(t, expRollups, rollups)
	orders.SortBucketBandwidthRollups(rollups)
	assert.Equal(t, expRollups, rollups)
}
