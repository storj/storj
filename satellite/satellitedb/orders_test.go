// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"storj.io/common/pb"
	"storj.io/common/uuid"
)

func TestSortRollupKeys(t *testing.T) {
	rollups := []bandwidthRollupKey{
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

	expRollups := []bandwidthRollupKey{
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
	sortBandwidthRollupKeys(rollups)
	assert.Empty(t, cmp.Diff(expRollups, rollups))
}
