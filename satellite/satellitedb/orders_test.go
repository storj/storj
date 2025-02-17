// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestSortRollupKeys(t *testing.T) {
	rollups := []satellitedb.BandwidthRollupKey{
		{
			ProjectID:     uuid.UUID{1},
			BucketName:    "a",
			IntervalStart: 1,
			Action:        pb.PieceAction_GET, // GET is 2

		},
		{
			ProjectID:     uuid.UUID{2},
			BucketName:    "a",
			IntervalStart: 2,
			Action:        pb.PieceAction_GET,
		},
		{
			ProjectID:     uuid.UUID{1},
			BucketName:    "b",
			IntervalStart: 3,
			Action:        pb.PieceAction_GET,
		},
		{
			ProjectID:     uuid.UUID{1},
			BucketName:    "a",
			IntervalStart: 4,
			Action:        pb.PieceAction_GET_AUDIT,
		},
		{
			ProjectID:     uuid.UUID{1},
			BucketName:    "a",
			IntervalStart: 5,
			Action:        pb.PieceAction_GET,
		},
	}

	expRollups := []satellitedb.BandwidthRollupKey{
		{
			ProjectID:     uuid.UUID{1},
			BucketName:    "a",
			IntervalStart: 1,
			Action:        pb.PieceAction_GET, // GET is 2
		},
		{
			ProjectID:     uuid.UUID{1},
			BucketName:    "a",
			IntervalStart: 4,
			Action:        pb.PieceAction_GET_AUDIT,
		},
		{
			ProjectID:     uuid.UUID{1},
			BucketName:    "a",
			IntervalStart: 5,
			Action:        pb.PieceAction_GET,
		},
		{
			ProjectID:     uuid.UUID{1},
			BucketName:    "b",
			IntervalStart: 3,
			Action:        pb.PieceAction_GET,
		},
		{
			ProjectID:     uuid.UUID{2},
			BucketName:    "a",
			IntervalStart: 2,
			Action:        pb.PieceAction_GET,
		},
	}

	assert.NotEqual(t, expRollups, rollups)
	satellitedb.SortBandwidthRollupKeys(rollups)
	assert.Empty(t, cmp.Diff(expRollups, rollups))
}

func TestUpdateBucketBandwidthAllocation(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {

		projectID := testrand.UUID()

		err := db.Orders().UpdateBucketBandwidthAllocation(ctx, projectID, []byte("bucket1"), pb.PieceAction_GET, 100, time.Date(2024, 01, 01, 12, 00, 0, 0, time.UTC))
		require.NoError(t, err)

		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, projectID, []byte("bucket1"), pb.PieceAction_GET, 200, time.Date(2024, 01, 01, 12, 01, 0, 0, time.UTC))
		require.NoError(t, err)

		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, projectID, []byte("bucket1"), pb.PieceAction_GET, 200, time.Date(2024, 01, 02, 13, 01, 0, 0, time.UTC))
		require.NoError(t, err)

		from := time.Date(2024, 01, 01, 00, 00, 0, 0, time.UTC)
		to := time.Date(2024, 01, 02, 00, 00, 0, 0, time.UTC)

		allocated, inline, settled, err := db.Orders().TestGetBucketBandwidth(ctx, projectID, []byte("bucket1"), from, to)
		require.NoError(t, err)
		require.Equal(t, int64(300), allocated)
		require.Equal(t, int64(0), inline)
		require.Equal(t, int64(0), settled)

	})
}

func TestUpdateBucketBandwidthSettle(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {

		projectID := testrand.UUID()

		err := db.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte("bucket1"), pb.PieceAction_GET, 100, 23, time.Date(2024, 01, 01, 12, 00, 0, 0, time.UTC))
		require.NoError(t, err)

		err = db.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte("bucket1"), pb.PieceAction_GET, 200, 0, time.Date(2024, 01, 01, 12, 01, 0, 0, time.UTC))
		require.NoError(t, err)

		err = db.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte("bucket1"), pb.PieceAction_GET, 200, 400, time.Date(2024, 01, 02, 13, 01, 0, 0, time.UTC))
		require.NoError(t, err)

		from := time.Date(2024, 01, 01, 00, 00, 0, 0, time.UTC)
		to := time.Date(2024, 01, 02, 00, 00, 0, 0, time.UTC)

		allocated, inline, settled, err := db.Orders().TestGetBucketBandwidth(ctx, projectID, []byte("bucket1"), from, to)
		require.NoError(t, err)
		require.Equal(t, int64(0), allocated)
		require.Equal(t, int64(0), inline)
		require.Equal(t, int64(300), settled)

	})
}

func TestUpdateBucketBandwidthInline(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {

		projectID := testrand.UUID()

		err := db.Orders().UpdateBucketBandwidthInline(ctx, projectID, []byte("bucket1"), pb.PieceAction_GET, 100, time.Date(2024, 01, 01, 12, 00, 0, 0, time.UTC))
		require.NoError(t, err)

		err = db.Orders().UpdateBucketBandwidthInline(ctx, projectID, []byte("bucket1"), pb.PieceAction_GET, 200, time.Date(2024, 01, 01, 12, 01, 0, 0, time.UTC))
		require.NoError(t, err)

		err = db.Orders().UpdateBucketBandwidthInline(ctx, projectID, []byte("bucket1"), pb.PieceAction_GET, 200, time.Date(2024, 01, 02, 13, 01, 0, 0, time.UTC))
		require.NoError(t, err)

		from := time.Date(2024, 01, 01, 00, 00, 0, 0, time.UTC)
		to := time.Date(2024, 01, 02, 00, 00, 0, 0, time.UTC)

		allocated, inline, settled, err := db.Orders().TestGetBucketBandwidth(ctx, projectID, []byte("bucket1"), from, to)
		require.NoError(t, err)
		require.Equal(t, int64(0), allocated)
		require.Equal(t, int64(300), inline)
		require.Equal(t, int64(0), settled)

	})
}

func TestUpdateStoragenodeBandwidthSettle(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {

		nodeID := storj.NodeID{}

		err := db.Orders().UpdateStoragenodeBandwidthSettle(ctx, nodeID, pb.PieceAction_GET, 100, time.Date(2024, 01, 01, 12, 00, 0, 0, time.UTC))
		require.NoError(t, err)

		err = db.Orders().UpdateStoragenodeBandwidthSettle(ctx, nodeID, pb.PieceAction_GET, 200, time.Date(2024, 01, 01, 12, 01, 0, 0, time.UTC))
		require.NoError(t, err)

		err = db.Orders().UpdateStoragenodeBandwidthSettle(ctx, nodeID, pb.PieceAction_GET, 200, time.Date(2024, 01, 02, 13, 01, 0, 0, time.UTC))
		require.NoError(t, err)

		from := time.Date(2024, 01, 01, 00, 00, 0, 0, time.UTC)
		to := time.Date(2024, 01, 02, 00, 00, 0, 0, time.UTC)

		res, err := db.Orders().GetStorageNodeBandwidth(ctx, nodeID, from, to)
		require.NoError(t, err)
		require.Equal(t, int64(300), res)

	})
}

func TestUpdateBandwidthBatch(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {

		from := time.Date(2024, 01, 01, 00, 00, 0, 0, time.UTC)
		to := time.Date(2024, 01, 02, 00, 00, 0, 0, time.UTC)

		var rollups []orders.BucketBandwidthRollup

		projectID := uuid.UUID{1}
		bucketName := "a"

		rollups = append(rollups, orders.BucketBandwidthRollup{
			ProjectID:     projectID,
			BucketName:    bucketName,
			Action:        pb.PieceAction_GET,
			IntervalStart: from.Add(1 * time.Hour),
			Inline:        100,
			Allocated:     0,
			Settled:       20,
			Dead:          0,
		})

		err := db.Orders().UpdateBandwidthBatch(ctx, rollups)
		require.NoError(t, err)

		allocated, inline, settled, err := db.Orders().TestGetBucketBandwidth(ctx, projectID, []byte(bucketName), from, to)
		require.NoError(t, err)
		require.Equal(t, int64(0), allocated)
		require.Equal(t, int64(100), inline)
		require.Equal(t, int64(20), settled)

	})
}

func TestUpdateBandwidthBatch_partialUpdate(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {

		from := time.Date(2024, 01, 01, 00, 00, 0, 0, time.UTC)
		to := time.Date(2024, 01, 02, 00, 00, 0, 0, time.UTC)

		var rollups []orders.BucketBandwidthRollup

		projectID := uuid.UUID{1}
		bucketName1 := "a"
		bucketName2 := "b"

		// first let's insert one record
		rollups = append(rollups, orders.BucketBandwidthRollup{
			ProjectID:     projectID,
			BucketName:    bucketName1,
			Action:        pb.PieceAction_GET,
			IntervalStart: from.Add(1 * time.Hour),
			Inline:        100,
			Allocated:     0,
			Settled:       13,
			Dead:          0,
		})

		err := db.Orders().UpdateBandwidthBatch(ctx, rollups)
		require.NoError(t, err)

		rollups = rollups[:0]
		// now, insert two records. One which updates the previous one, one which is new
		rollups = append(rollups, orders.BucketBandwidthRollup{
			ProjectID:     projectID,
			BucketName:    bucketName1,
			Action:        pb.PieceAction_GET,
			IntervalStart: from.Add(1 * time.Hour),
			Inline:        200,
			Allocated:     0,
			Settled:       20,
			Dead:          0,
		})
		rollups = append(rollups, orders.BucketBandwidthRollup{
			ProjectID:     projectID,
			BucketName:    bucketName2,
			Action:        pb.PieceAction_GET,
			IntervalStart: from.Add(1 * time.Hour),
			Inline:        55,
			Allocated:     0,
			Settled:       10,
			Dead:          0,
		})

		err = db.Orders().UpdateBandwidthBatch(ctx, rollups)
		require.NoError(t, err)

		// first record
		allocated, inline, settled, err := db.Orders().TestGetBucketBandwidth(ctx, projectID, []byte(bucketName1), from, to)
		require.NoError(t, err)
		require.Equal(t, int64(0), allocated)
		require.Equal(t, int64(300), inline)
		require.Equal(t, int64(33), settled)

		// second record
		allocated, inline, settled, err = db.Orders().TestGetBucketBandwidth(ctx, projectID, []byte(bucketName2), from, to)
		require.NoError(t, err)
		require.Equal(t, int64(0), allocated)
		require.Equal(t, int64(55), inline)
		require.Equal(t, int64(10), settled)

	})
}
