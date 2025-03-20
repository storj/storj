// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue_test

import (
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/errs2"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairqueuetest"
)

func TestInsertSelect(t *testing.T) {
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
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
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
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
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
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
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
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
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
		_, err := q.Select(ctx, 1, nil, nil)
		require.Error(t, err)
		require.True(t, queue.ErrEmpty.Has(err), "error should of class EmptyQueue")
	})
}

func TestSequential(t *testing.T) {
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
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
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
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

func TestClean(t *testing.T) {
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, q queue.RepairQueue) {
		// Create three segments
		seg1 := &queue.InjuredSegment{
			StreamID: testrand.UUID(),
		}
		seg2 := &queue.InjuredSegment{
			StreamID: testrand.UUID(),
		}
		seg3 := &queue.InjuredSegment{
			StreamID: testrand.UUID(),
		}

		// Create reference time before insertion
		timeBeforeInsert := time.Now().Add(-time.Hour)

		// Insert all segments - this will set their UpdatedAt times to now
		segmentHealth := 1.3
		seg1.SegmentHealth = segmentHealth
		_, err := q.Insert(ctx, seg1)
		require.NoError(t, err)

		seg2.SegmentHealth = segmentHealth
		_, err = q.Insert(ctx, seg2)
		require.NoError(t, err)

		seg3.SegmentHealth = segmentHealth
		_, err = q.Insert(ctx, seg3)
		require.NoError(t, err)

		// mark seg1 as updated an hour ago
		_, err = q.TestingSetUpdatedTime(ctx, 0, seg1.StreamID, seg1.Position, timeBeforeInsert)
		require.NoError(t, err)

		count, err := q.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 3, count)

		// Clean should not remove any segments when using a time before all segments
		d, err := q.Clean(ctx, timeBeforeInsert.Add(-time.Hour))
		require.NoError(t, err)
		require.Equal(t, int64(0), d)

		// Clean should remove 1 segment (seg1) when using timeBeforeInsert
		d, err = q.Clean(ctx, timeBeforeInsert.Add(time.Minute))
		require.NoError(t, err)
		require.Equal(t, int64(1), d)

		// We should have 2 segments left
		count, err = q.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		// Clean with current time should remove all segments
		d, err = q.Clean(ctx, time.Now().Add(time.Minute))
		require.NoError(t, err)
		require.Equal(t, int64(2), d)

		// We should have 0 segments left
		count, err = q.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})
}
