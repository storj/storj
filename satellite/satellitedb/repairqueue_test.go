// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

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

		rs1, err := rq.Select(ctx, 10, nil, nil)
		require.NoError(t, err)
		require.Len(t, rs1, 1)
		require.Equal(t, testSegments[0].StreamID, rs1[0].StreamID)
		require.Equal(t, storj.PlacementConstraint(99), rs1[0].Placement)
		require.Equal(t, testSegments[0].Position, rs1[0].Position)
		require.Equal(t, float64(12), rs1[0].SegmentHealth)
		err = rq.Release(ctx, rs1[0], true)
		require.NoError(t, err)

		// empty queue (one record, but that's already attempted)
		_, err = rq.Select(ctx, 1, nil, nil)
		require.Error(t, err)

		// make sure it's really empty
		err = rq.Delete(ctx, *testSegments[0])
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
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
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

		rq := db.RepairQueue()

		for i := 0; i < len(testSegments); i++ {
			inserted, err := rq.Insert(ctx, testSegments[i])
			require.NoError(t, err)
			require.False(t, inserted)
		}

		// any random segment
		randomSegments, err := rq.Select(ctx, 1, nil, nil)
		require.NoError(t, err)
		require.Len(t, randomSegments, 1)
		err = rq.Release(ctx, randomSegments[0], true)
		require.NoError(t, err)

		for i := 0; i < 2; i++ {
			// placement constraint
			selected, err := rq.Select(ctx, 1, []storj.PlacementConstraint{1, 2}, nil)
			require.NoError(t, err)
			require.True(t, selected[0].Placement == 1 || selected[0].Placement == 2, "Expected placement 1 or 2 but was %d", selected[0].Placement)
			err = rq.Release(ctx, selected[0], true)
			require.NoError(t, err)

			selected, err = rq.Select(ctx, 1, []storj.PlacementConstraint{3, 4}, []storj.PlacementConstraint{3})
			require.NoError(t, err)
			require.Equal(t, storj.PlacementConstraint(4), selected[0].Placement)
			err = rq.Release(ctx, selected[0], true)
			require.NoError(t, err)

			selected, err = rq.Select(ctx, 1, nil, []storj.PlacementConstraint{0, 1, 2, 3, 4})
			require.NoError(t, err)
			require.True(t, selected[0].Placement > 4)
			err = rq.Release(ctx, selected[0], true)
			require.NoError(t, err)

			// the Select above does not order by the primary key, so it may update the segment with placement constraint 9.
			// if so, explicitly update a different segment that is not yet updated (such as 8)
			singlePlacement := storj.PlacementConstraint(9)
			if selected[0].Placement == singlePlacement {
				singlePlacement = storj.PlacementConstraint(8)
			}
			selected, err = rq.Select(ctx, 1, []storj.PlacementConstraint{singlePlacement}, []storj.PlacementConstraint{1, 2, 3, 4})
			require.NoError(t, err)
			require.Equal(t, singlePlacement, selected[0].Placement)
			err = rq.Release(ctx, selected[0], true)
			require.NoError(t, err)

			_, err = rq.Select(ctx, 1, []storj.PlacementConstraint{11}, nil)
			require.Error(t, err)
		}

	})
}

func TestRepairQueue_BatchInsert(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		testSegments := make([]*queue.InjuredSegment, 5)
		for i := 0; i < len(testSegments); i++ {

			placement := storj.PlacementConstraint(i % 10)
			uuid := testrand.UUID()
			uuid[0] = byte(placement)

			testSegments[i] = &queue.InjuredSegment{
				StreamID: uuid,
				Position: metabase.SegmentPosition{
					Part:  uint32(i),
					Index: 2,
				},
				SegmentHealth: 10,
				Placement:     placement,
			}
		}

		// fresh inserts
		rq := db.RepairQueue()
		_, err := rq.InsertBatch(ctx, testSegments)
		require.NoError(t, err)

		for i := 0; i < len(testSegments); i++ {
			segments, err := rq.Select(ctx, 1, []storj.PlacementConstraint{storj.PlacementConstraint(i)}, nil)
			require.NoError(t, err)
			assert.Equal(t, storj.PlacementConstraint(segments[0].StreamID[0]), segments[0].Placement)
			err = rq.Release(ctx, segments[0], false)
			require.NoError(t, err)
		}

		// fresh inserts again
		_, err = rq.InsertBatch(ctx, testSegments)
		require.NoError(t, err)

		for _, ts := range testSegments {
			ts.StreamID[0] = byte(ts.Placement + 1)
		}

		// this time placement is changed between inserts.
		_, err = rq.InsertBatch(ctx, testSegments)
		require.NoError(t, err)

		for i := 0; i < len(testSegments); i++ {
			segments, err := rq.Select(ctx, 1, []storj.PlacementConstraint{storj.PlacementConstraint(i)}, nil)
			require.NoError(t, err)
			require.Equal(t, storj.PlacementConstraint(segments[0].StreamID[0]), segments[0].Placement+1)
			err = rq.Release(ctx, segments[0], false)
			require.NoError(t, err)
		}
	})
}

func TestRepairQueue_Stat(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		testSegments := make([]*queue.InjuredSegment, 20)
		for i := 0; i < len(testSegments); i++ {

			placement := storj.PlacementConstraint(i % 5)
			uuid := testrand.UUID()
			uuid[0] = byte(placement)

			is := &queue.InjuredSegment{
				StreamID: uuid,
				Position: metabase.SegmentPosition{
					Part:  uint32(i),
					Index: 2,
				},
				SegmentHealth: 10,
				Placement:     placement,
			}
			testSegments[i] = is
		}
		rq := db.RepairQueue()

		_, err := rq.InsertBatch(ctx, testSegments)
		require.NoError(t, err)

		job, err := rq.Select(ctx, 1, nil, nil)
		require.NoError(t, err)
		err = rq.Release(ctx, job[0], false)
		require.NoError(t, err)

		stat, err := rq.Stat(ctx)
		require.NoError(t, err)

		// we have 5 placement, but one has both attempted and non-attempted entries
		require.Len(t, stat, 6)
	})
}

func TestRepairQueue_Select_Concurrently(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		expectedSegments := make([]*queue.InjuredSegment, 100)
		segs := make([]queue.InjuredSegment, len(expectedSegments))
		for i := 0; i < len(expectedSegments); i++ {

			placement := storj.PlacementConstraint(i % 5)
			uuid := testrand.UUID()
			uuid[0] = byte(placement)

			is := queue.InjuredSegment{
				StreamID: uuid,
				Position: metabase.SegmentPosition{
					Part:  uint32(i),
					Index: 2,
				},
				SegmentHealth: 10,
				Placement:     placement,
			}
			segs[i] = is
			expectedSegments[i] = &is
		}

		rq := db.RepairQueue()

		_, err := rq.InsertBatch(ctx, expectedSegments)
		require.NoError(t, err)

		segments, err := rq.SelectN(ctx, len(expectedSegments))
		require.NoError(t, err)
		require.Len(t, segments, len(expectedSegments))

		mu := sync.Mutex{}
		selectedSegments := []queue.InjuredSegment{}

		parallel := 5
		if os.Getenv("STORJ_TEST_ENVIRONMENT") == "spanner-nightly" {
			parallel = 2
		}

		group := errgroup.Group{}
		for i := 0; i < parallel; i++ {
			group.Go(func() error {
				segments := []queue.InjuredSegment{}
				for {
					result, err := rq.Select(ctx, 3, nil, nil)
					if queue.ErrEmpty.Has(err) {
						mu.Lock()
						selectedSegments = append(selectedSegments, segments...)
						mu.Unlock()
						return nil
					}
					if err != nil {
						return err
					}
					err = rq.Release(ctx, result[0], true)
					if err != nil {
						return err
					}

					segments = append(segments, result...)
				}
			})
		}
		require.NoError(t, group.Wait())
		require.Len(t, selectedSegments, len(expectedSegments))

		for i := range segments {
			segments[i].UpdatedAt = time.Time{}
			selectedSegments[i].UpdatedAt = time.Time{}
			segments[i].AttemptedAt = nil
			selectedSegments[i].AttemptedAt = nil
		}

		require.ElementsMatch(t, segments, selectedSegments)
	})
}
