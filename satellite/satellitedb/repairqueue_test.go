// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestRepairQueue(t *testing.T) {
	testSegments := make([]*queue.InjuredSegment, 3)
	for i := 0; i < len(testSegments); i++ {
		testSegments[i] = &queue.InjuredSegment{
			StreamID: testrand.UUID(),
			Position: metabase.SegmentPosition{
				Part:  uint32(i),
				Index: 2,
			},
			SegmentHealth: 10,
			Placement:     storj.PlacementConstraint(i),
		}
	}

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		rq := db.RepairQueue()

		alreadyInserted, err := rq.Insert(ctx, testSegments[0])
		require.NoError(t, err)
		require.False(t, alreadyInserted)

		sn, err := rq.SelectN(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, testSegments[0].StreamID, sn[0].StreamID)
		require.Equal(t, testSegments[0].Placement, sn[0].Placement)
		require.Equal(t, testSegments[0].Position, sn[0].Position)
		require.Equal(t, testSegments[0].SegmentHealth, sn[0].SegmentHealth)

		// upsert
		alreadyInserted, err = rq.Insert(ctx, &queue.InjuredSegment{
			StreamID:      testSegments[0].StreamID,
			Position:      testSegments[0].Position,
			SegmentHealth: 12,
			Placement:     storj.PlacementConstraint(99),
		})
		require.NoError(t, err)
		require.True(t, alreadyInserted)

		rs1, err := rq.Select(ctx, nil, nil)
		require.NoError(t, err)
		require.Equal(t, testSegments[0].StreamID, rs1.StreamID)
		require.Equal(t, storj.PlacementConstraint(99), rs1.Placement)
		require.Equal(t, testSegments[0].Position, rs1.Position)
		require.Equal(t, float64(12), rs1.SegmentHealth)

		// empty queue (one record, but that's already attempted)
		_, err = rq.Select(ctx, nil, nil)
		require.Error(t, err)

		// make sure it's really empty
		err = rq.Delete(ctx, testSegments[0])
		require.NoError(t, err)

		// insert 2 new
		newlyInserted, err := rq.InsertBatch(ctx, []*queue.InjuredSegment{
			testSegments[1], testSegments[2],
		})
		require.NoError(t, err)
		require.Len(t, newlyInserted, 2)

		// select2 (including attempted)
		segments, err := rq.SelectN(ctx, 2)
		require.NoError(t, err)

		sort.Slice(segments, func(i, j int) bool {
			return segments[i].Position.Part < segments[j].Position.Part
		})

		for i := 0; i < 2; i++ {
			require.Equal(t, testSegments[i+1].StreamID, segments[i].StreamID)
			require.Equal(t, testSegments[i+1].Placement, segments[i].Placement)
			require.Equal(t, testSegments[i+1].Position, segments[i].Position)
			require.Equal(t, testSegments[i+1].SegmentHealth, segments[i].SegmentHealth)
		}

	})
}

func TestRepairQueue_PlacementRestrictions(t *testing.T) {
	testSegments := make([]*queue.InjuredSegment, 40)
	for i := 0; i < len(testSegments); i++ {
		testSegments[i] = &queue.InjuredSegment{
			StreamID: testrand.UUID(),
			Position: metabase.SegmentPosition{
				Part:  uint32(i),
				Index: 2,
			},
			SegmentHealth: 10,
			Placement:     storj.PlacementConstraint(i % 10),
		}
	}

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		rq := db.RepairQueue()

		for i := 0; i < len(testSegments); i++ {
			inserted, err := rq.Insert(ctx, testSegments[i])
			require.NoError(t, err)
			require.False(t, inserted)
		}

		// any random segment
		_, err := rq.Select(ctx, nil, nil)
		require.NoError(t, err)

		for i := 0; i < 2; i++ {
			// placement constraint
			selected, err := rq.Select(ctx, []storj.PlacementConstraint{1, 2}, nil)
			require.NoError(t, err)
			require.True(t, selected.Placement == 1 || selected.Placement == 2, "Expected placement 1 or 2 but was %d", selected.Placement)

			selected, err = rq.Select(ctx, []storj.PlacementConstraint{3, 4}, []storj.PlacementConstraint{3})
			require.NoError(t, err)
			require.Equal(t, storj.PlacementConstraint(4), selected.Placement)

			selected, err = rq.Select(ctx, nil, []storj.PlacementConstraint{0, 1, 2, 3, 4})
			require.NoError(t, err)
			require.True(t, selected.Placement > 4)

			selected, err = rq.Select(ctx, []storj.PlacementConstraint{9}, []storj.PlacementConstraint{1, 2, 3, 4})
			require.NoError(t, err)
			require.Equal(t, storj.PlacementConstraint(9), selected.Placement)

			_, err = rq.Select(ctx, []storj.PlacementConstraint{11}, nil)
			require.Error(t, err)
		}

	})
}
