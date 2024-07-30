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
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
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

		allocated, _, _, err := db.Orders().TestGetBucketBandwidth(ctx, projectID, []byte("bucket1"), from, to)
		require.NoError(t, err)
		require.Equal(t, int64(300), allocated)

	})
}
