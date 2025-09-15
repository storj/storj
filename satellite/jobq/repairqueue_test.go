// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobq_test

import (
	"math/rand"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/jobqtest"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/repair/queue"
)

// Tests adapted from satellite/satellitedb/repairqueue_test.go

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

	jobqtest.Run(t, func(ctx *testcontext.Context, t *testing.T, rq queue.RepairQueue) {
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
			Placement:     testSegments[0].Placement,
		})
		require.NoError(t, err)
		require.True(t, alreadyInserted)

		rs1, err := rq.Select(ctx, 10, nil, nil)
		require.NoError(t, err)
		require.Len(t, rs1, 1)
		require.Equal(t, testSegments[0].StreamID, rs1[0].StreamID)
		require.Equal(t, testSegments[0].Placement, rs1[0].Placement)
		require.Equal(t, testSegments[0].Position, rs1[0].Position)
		require.Equal(t, float64(12), rs1[0].SegmentHealth)
		err = rq.Release(ctx, rs1[0], false)
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

		// select 2 (not including attempted)
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
	jobqtest.Run(t, func(ctx *testcontext.Context, t *testing.T, rq queue.RepairQueue) {
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
	jobqtest.Run(t, func(ctx *testcontext.Context, t *testing.T, rq queue.RepairQueue) {
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
			ts.SegmentHealth = 5
		}

		// this time Health changed between inserts.
		_, err = rq.InsertBatch(ctx, testSegments)
		require.NoError(t, err)

		for i := 0; i < len(testSegments); i++ {
			segments, err := rq.Select(ctx, 1, []storj.PlacementConstraint{storj.PlacementConstraint(i)}, nil)
			require.NoError(t, err)
			require.Equal(t, 5.0, segments[0].SegmentHealth)
			err = rq.Release(ctx, segments[0], false)
			require.NoError(t, err)
		}
	})
}

func TestRepairQueue_Stat(t *testing.T) {
	jobqtest.WithServerAndClient(t, nil, func(ctx *testcontext.Context, srv *jobqtest.TestServer, cli *jobq.Client) {
		rq := jobq.WrapJobQueue(cli)
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
				SegmentHealth:            10,
				Placement:                placement,
				NumNormalizedHealthy:     5,
				NumNormalizedRetrievable: 6,
				NumOutOfPlacement:        7,
			}
			testSegments[i] = is
		}

		_, err := rq.InsertBatch(ctx, testSegments)
		require.NoError(t, err)

		job, err := rq.Select(ctx, 1, nil, nil)
		require.NoError(t, err)
		err = rq.Release(ctx, job[0], false)
		require.NoError(t, err)

		inspect, err := cli.Inspect(ctx, job[0].Placement, job[0].StreamID, job[0].Position.Encode())
		require.NoError(t, err)
		require.Equal(t, int16(5), inspect.NumNormalizedHealthy)
		require.Equal(t, int16(6), inspect.NumNormalizedRetrievable)
		require.Equal(t, int16(7), inspect.NumOutOfPlacement)

		stat, err := rq.Stat(ctx)
		require.NoError(t, err)

		// we have 5 placement, but one has both attempted and non-attempted entries
		require.Len(t, stat, 6)
	})
}

func TestRepairQueue_Select_Concurrently(t *testing.T) {
	jobqtest.Run(t, func(ctx *testcontext.Context, t *testing.T, rq queue.RepairQueue) {
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

// Tests adapted from satellite/satellitedb/queue_test.go

func TestInsertSelect(t *testing.T) {
	jobqtest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
		seg := createInjuredSegment()
		seg.SegmentHealth = 0.4

		alreadyInserted, err := q.Insert(ctx, seg)
		require.NoError(t, err)
		require.False(t, alreadyInserted)
		segments, err := q.Select(ctx, 1, nil, nil)
		require.NoError(t, err)
		err = q.Release(ctx, segments[0], true)
		require.NoError(t, err)
		require.Equal(t, seg.StreamID, segments[0].StreamID)
		require.Equal(t, seg.Position, segments[0].Position)
		require.Equal(t, seg.SegmentHealth, segments[0].SegmentHealth)
		require.WithinDuration(t, time.Now(), segments[0].InsertedAt, 5*time.Second)
		require.NotZero(t, segments[0].UpdatedAt)
	})
}

func TestInsertDuplicate(t *testing.T) {
	jobqtest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
		seg := createInjuredSegment()
		alreadyInserted, err := q.Insert(ctx, seg)
		require.NoError(t, err)
		require.False(t, alreadyInserted)
		alreadyInserted, err = q.Insert(ctx, seg)
		require.NoError(t, err)
		require.True(t, alreadyInserted)
	})
}

func TestInsertBatchOfOne(t *testing.T) {
	jobqtest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
		writeSegments := []*queue.InjuredSegment{
			createInjuredSegment(),
		}
		newlyInserted, err := q.InsertBatch(ctx, writeSegments)
		require.NoError(t, err)
		require.Len(t, newlyInserted, 1)

		writeSegments[0].SegmentHealth = 5
		newlyInserted, err = q.InsertBatch(ctx, writeSegments)
		require.NoError(t, err)
		require.Len(t, newlyInserted, 0)

		readSegments, err := q.SelectN(ctx, 1000)
		require.NoError(t, err)
		require.Len(t, readSegments, 1)
		require.Equal(t, writeSegments[0].StreamID, readSegments[0].StreamID)
		require.Equal(t, writeSegments[0].Position, readSegments[0].Position)
		require.Equal(t, writeSegments[0].SegmentHealth, readSegments[0].SegmentHealth)
		require.Equal(t, writeSegments[0].Placement, readSegments[0].Placement)
	})
}

func TestInsertOverlappingBatches(t *testing.T) {
	jobqtest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
		requireDbState := func(expectedSegments []queue.InjuredSegment) {
			sort := func(segments []queue.InjuredSegment) {
				sort.Slice(segments, func(i, j int) bool {
					return segments[i].StreamID.Less(segments[j].StreamID)
				})
			}

			dbSegments, err := q.SelectN(ctx, 1000)
			require.NoError(t, err)

			sort(dbSegments)
			sort(expectedSegments)

			require.Equal(t, len(expectedSegments), len(dbSegments))

			for i := range expectedSegments {
				require.Equal(t, expectedSegments[i].StreamID, dbSegments[i].StreamID)
			}
		}

		writeSegment1 := createInjuredSegment()
		writeSegment2 := createInjuredSegment()
		writeSegment3 := createInjuredSegment()

		newlyInserted, err := q.InsertBatch(ctx, []*queue.InjuredSegment{writeSegment1, writeSegment2})
		require.NoError(t, err)
		require.Len(t, newlyInserted, 2)
		require.Equal(t, newlyInserted[0], writeSegment1)
		require.Equal(t, newlyInserted[1], writeSegment2)
		requireDbState([]queue.InjuredSegment{*writeSegment1, *writeSegment2})

		newlyInserted, err = q.InsertBatch(ctx, []*queue.InjuredSegment{writeSegment2, writeSegment3})
		require.NoError(t, err)
		require.Len(t, newlyInserted, 1)
		require.Equal(t, newlyInserted[0], writeSegment3)
		requireDbState([]queue.InjuredSegment{*writeSegment1, *writeSegment2, *writeSegment3})

		newlyInserted, err = q.InsertBatch(ctx, []*queue.InjuredSegment{writeSegment1, writeSegment3})
		require.NoError(t, err)
		require.Len(t, newlyInserted, 0)
		requireDbState([]queue.InjuredSegment{*writeSegment1, *writeSegment2, *writeSegment3})
	})
}

func TestDequeueEmptyQueue(t *testing.T) {
	jobqtest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
		_, err := q.Select(ctx, 1, nil, nil)
		require.Error(t, err)
		require.True(t, queue.ErrEmpty.Has(err), "error should of class EmptyQueue")
	})
}

func TestSequential(t *testing.T) {
	jobqtest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
		const N = 20
		var added []*queue.InjuredSegment
		for i := 0; i < N; i++ {
			seg := &queue.InjuredSegment{
				StreamID:      uuid.UUID{byte(i)},
				SegmentHealth: 6,
			}
			alreadyInserted, err := q.Insert(ctx, seg)
			require.NoError(t, err)
			require.False(t, alreadyInserted)
			added = append(added, seg)
		}

		list, err := q.SelectN(ctx, N)
		require.NoError(t, err)
		require.Len(t, list, N)

		got := []*queue.InjuredSegment{}
		for {
			s, err := q.Select(ctx, 1, nil, nil)
			if queue.ErrEmpty.Has(err) {
				break
			}
			require.NoError(t, err)
			require.Len(t, s, 1)
			err = q.Release(ctx, s[0], true)
			require.NoError(t, err)

			got = append(got, &s[0])
		}

		sort.Slice(got, func(i, j int) bool {
			return got[i].StreamID.Less(got[j].StreamID)
		})

		require.Equal(t, len(added), len(got))
		for i, add := range added {
			assert.Equal(t, add.StreamID, got[i].StreamID, i)
			assert.Equal(t, add.Position, got[i].Position, i)
			assert.Equal(t, add.SegmentHealth, got[i].SegmentHealth, i)
		}
	})
}

func TestParallel(t *testing.T) {
	jobqtest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
		const N = 20

		expectedSegments := make([]queue.InjuredSegment, N)
		for i := 0; i < N; i++ {
			expectedSegments[i] = queue.InjuredSegment{
				StreamID:      testrand.UUID(),
				SegmentHealth: float64(i),
			}
		}

		var inserts errs2.Group
		// Add to queue concurrently
		for i := 0; i < N; i++ {
			i := i
			inserts.Go(func() error {
				alreadyInserted, err := q.Insert(ctx, &expectedSegments[i])
				require.False(t, alreadyInserted)

				// just to make expectedSegments match the values to be retrieved
				now := time.Now()
				expectedSegments[i].AttemptedAt = &now
				expectedSegments[i].InsertedAt = now
				expectedSegments[i].UpdatedAt = now
				return err
			})
		}
		require.Empty(t, inserts.Wait(), "unexpected queue.Insert errors")

		count, err := q.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, N, count)

		// Remove from queue concurrently
		items := make([]queue.InjuredSegment, N)
		var remove errs2.Group
		for i := 0; i < N; i++ {
			i := i
			remove.Go(func() error {
				s, err := q.Select(ctx, 1, nil, nil)
				if err != nil {
					return err
				}
				if len(s) != 1 {
					return errs.New("got %d segments, expected 1: %+v", len(s), s)
				}

				err = q.Release(ctx, s[0], true)
				if err != nil {
					return err
				}

				items[i] = s[0]
				return nil
			})
		}

		require.Empty(t, remove.Wait(), "unexpected queue.Select/Delete errors")

		sort.Slice(items, func(i, k int) bool {
			return items[i].SegmentHealth < items[k].SegmentHealth
		})

		// check if the enqueued and dequeued elements match
		diff := cmp.Diff(expectedSegments, items, cmpopts.EquateApproxTime(time.Hour))
		require.Zero(t, diff)

		count, err = q.Count(ctx)
		require.NoError(t, err)
		require.Zero(t, count)
	})
}

func createInjuredSegment() *queue.InjuredSegment {
	index := uint32(testrand.Intn(1000))
	return &queue.InjuredSegment{
		StreamID: testrand.UUID(),
		Position: metabase.SegmentPosition{
			Part:  uint32(testrand.Intn(1000)),
			Index: index,
		},
		SegmentHealth: 10,
		Placement:     storj.PlacementConstraint(index % 3),
	}
}

// TestRepairQueue_HealthOrder tests that segments are properly ordered by health,
// including negative health values which can occur when nodes perform graceful exit,
// are clumped together, or are out of placement. In these cases, the pieces can
// still be downloaded but are considered unhealthy.
func TestRepairQueue_HealthOrder(t *testing.T) {
	jobqtest.Run(t, func(ctx *testcontext.Context, t *testing.T, rq queue.RepairQueue) {
		segments := []*queue.InjuredSegment{
			{
				StreamID:      testrand.UUID(),
				Position:      metabase.SegmentPosition{Part: 0},
				SegmentHealth: -2.5, // Very negative health (highest priority)
				Placement:     storj.PlacementConstraint(1),
			},
			{
				StreamID:      testrand.UUID(),
				Position:      metabase.SegmentPosition{Part: 1},
				SegmentHealth: -0.5, // Slightly negative health
				Placement:     storj.PlacementConstraint(1),
			},
			{
				StreamID:      testrand.UUID(),
				Position:      metabase.SegmentPosition{Part: 2},
				SegmentHealth: 0.0, // Zero health
				Placement:     storj.PlacementConstraint(1),
			},
			{
				StreamID:      testrand.UUID(),
				Position:      metabase.SegmentPosition{Part: 3},
				SegmentHealth: 1.5, // Positive health
				Placement:     storj.PlacementConstraint(1),
			},
			{
				StreamID:      testrand.UUID(),
				Position:      metabase.SegmentPosition{Part: 4},
				SegmentHealth: 3.0, // Higher positive health
				Placement:     storj.PlacementConstraint(1),
			},
		}

		// Insert all segments in random order to ensure ordering is based on health
		rand.Shuffle(len(segments), func(i, j int) {
			segments[i], segments[j] = segments[j], segments[i]
		})

		_, err := rq.InsertBatch(ctx, segments)
		require.NoError(t, err)

		// Verify count
		count, err := rq.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, len(segments), count)

		// Pop segments and verify order
		var poppedSegments []queue.InjuredSegment
		for i := 0; i < len(segments); i++ {
			seg, err := rq.Select(ctx, 1, nil, nil)
			require.NoError(t, err)
			require.Len(t, seg, 1)
			poppedSegments = append(poppedSegments, seg[0])
			err = rq.Release(ctx, seg[0], true)
			require.NoError(t, err)
		}

		// Verify segments were popped in order of ascending health
		require.Equal(t, len(segments), len(poppedSegments))
		for i := 1; i < len(poppedSegments); i++ {
			require.GreaterOrEqual(t,
				poppedSegments[i].SegmentHealth,
				poppedSegments[i-1].SegmentHealth,
				"segments should be popped in ascending health order")
		}

		// Verify specific health values for edge cases
		require.Equal(t, -2.5, poppedSegments[0].SegmentHealth, "first segment should have lowest health")
		require.Equal(t, 3.0, poppedSegments[len(poppedSegments)-1].SegmentHealth, "last segment should have highest health")

		// Verify queue is empty
		_, err = rq.Select(ctx, 1, nil, nil)
		require.True(t, queue.ErrEmpty.Has(err))
	})
}
