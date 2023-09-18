// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue_test

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/errs2"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestInsertSelect(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()

		seg := createInjuredSegment()
		seg.SegmentHealth = 0.4

		alreadyInserted, err := q.Insert(ctx, seg)
		require.NoError(t, err)
		require.False(t, alreadyInserted)
		s, err := q.Select(ctx)
		require.NoError(t, err)
		err = q.Delete(ctx, s)
		require.NoError(t, err)
		require.Equal(t, seg.StreamID, s.StreamID)
		require.Equal(t, seg.Position, s.Position)
		require.Equal(t, seg.SegmentHealth, s.SegmentHealth)
		require.WithinDuration(t, time.Now(), s.InsertedAt, 5*time.Second)
		require.NotZero(t, s.UpdatedAt)
	})
}

func TestInsertDuplicate(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()

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
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()

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
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()

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
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()

		_, err := q.Select(ctx)
		require.Error(t, err)
		require.True(t, queue.ErrEmpty.Has(err), "error should of class EmptyQueue")
	})
}

func TestSequential(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()

		const N = 20
		var addSegs []*queue.InjuredSegment
		for i := 0; i < N; i++ {
			seg := &queue.InjuredSegment{
				StreamID:      uuid.UUID{byte(i)},
				SegmentHealth: 6,
			}
			alreadyInserted, err := q.Insert(ctx, seg)
			require.NoError(t, err)
			require.False(t, alreadyInserted)
			addSegs = append(addSegs, seg)
		}

		list, err := q.SelectN(ctx, N)
		require.NoError(t, err)
		require.Len(t, list, N)

		for i := 0; i < N; i++ {
			s, err := q.Select(ctx)
			require.NoError(t, err)
			err = q.Delete(ctx, s)
			require.NoError(t, err)

			require.Equal(t, addSegs[i].StreamID, s.StreamID)
			require.Equal(t, addSegs[i].Position, s.Position)
			require.Equal(t, addSegs[i].SegmentHealth, s.SegmentHealth)
		}
	})
}

func TestParallel(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()
		const N = 20
		entries := make(chan *queue.InjuredSegment, N)

		expectedSegments := make([]*queue.InjuredSegment, N)
		for i := 0; i < N; i++ {
			expectedSegments[i] = &queue.InjuredSegment{
				StreamID:      testrand.UUID(),
				SegmentHealth: float64(i),
			}
		}

		var inserts errs2.Group
		// Add to queue concurrently
		for i := 0; i < N; i++ {
			i := i
			inserts.Go(func() error {
				alreadyInserted, err := q.Insert(ctx, expectedSegments[i])
				require.False(t, alreadyInserted)
				return err
			})
		}
		require.Empty(t, inserts.Wait(), "unexpected queue.Insert errors")

		// Remove from queue concurrently
		var remove errs2.Group
		for i := 0; i < N; i++ {
			remove.Go(func() error {
				s, err := q.Select(ctx)
				if err != nil {
					return err
				}

				err = q.Delete(ctx, s)
				if err != nil {
					return err
				}

				entries <- s
				return nil
			})
		}

		require.Empty(t, remove.Wait(), "unexpected queue.Select/Delete errors")
		close(entries)

		var items []*queue.InjuredSegment
		for segment := range entries {
			items = append(items, segment)
		}

		sort.Slice(items, func(i, k int) bool {
			return items[i].SegmentHealth < items[k].SegmentHealth
		})

		// check if the enqueued and dequeued elements match
		for i := 0; i < N; i++ {
			require.Equal(t, expectedSegments[i].StreamID, items[i].StreamID)
			require.Equal(t, expectedSegments[i].Position, items[i].Position)
			require.Equal(t, expectedSegments[i].SegmentHealth, items[i].SegmentHealth)
		}

		count, err := q.Count(ctx)
		require.NoError(t, err)
		require.Zero(t, count)
	})
}

func TestClean(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()

		seg1 := &queue.InjuredSegment{
			StreamID: testrand.UUID(),
		}
		seg2 := &queue.InjuredSegment{
			StreamID: testrand.UUID(),
		}
		seg3 := &queue.InjuredSegment{
			StreamID: testrand.UUID(),
		}

		timeBeforeInsert1 := time.Now()

		segmentHealth := 1.3
		_, err := q.Insert(ctx, seg1)
		require.NoError(t, err)

		_, err = q.Insert(ctx, seg2)
		require.NoError(t, err)

		_, err = q.Insert(ctx, seg3)
		require.NoError(t, err)

		count, err := q.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 3, count)

		d, err := q.Clean(ctx, timeBeforeInsert1)
		require.NoError(t, err)
		require.Equal(t, int64(0), d)

		count, err = q.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 3, count)

		timeBeforeInsert2 := time.Now()

		// seg1 "becomes healthy", so do not update it
		// seg2 stays at the same health
		_, err = q.Insert(ctx, seg2)
		require.NoError(t, err)

		// seg3 has a lower health
		seg3.SegmentHealth = segmentHealth - 0.1
		_, err = q.Insert(ctx, seg3)
		require.NoError(t, err)

		count, err = q.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 3, count)

		d, err = q.Clean(ctx, timeBeforeInsert2)
		require.NoError(t, err)
		require.Equal(t, int64(1), d)

		count, err = q.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		d, err = q.Clean(ctx, time.Now())
		require.NoError(t, err)
		require.Equal(t, int64(2), d)

		count, err = q.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})
}
