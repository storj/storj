// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue_test

import (
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage"
)

func TestInsertSelect(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()

		seg := &internalpb.InjuredSegment{
			Path:       []byte("abc"),
			LostPieces: []int32{int32(1), int32(3)},
		}
		alreadyInserted, err := q.Insert(ctx, seg, 10)
		require.NoError(t, err)
		require.False(t, alreadyInserted)
		s, err := q.Select(ctx)
		require.NoError(t, err)
		err = q.Delete(ctx, s)
		require.NoError(t, err)
		require.True(t, pb.Equal(s, seg))
	})
}

func TestInsertDuplicate(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()

		seg := &internalpb.InjuredSegment{
			Path:       []byte("abc"),
			LostPieces: []int32{int32(1), int32(3)},
		}
		alreadyInserted, err := q.Insert(ctx, seg, 10)
		require.NoError(t, err)
		require.False(t, alreadyInserted)
		alreadyInserted, err = q.Insert(ctx, seg, 10)
		require.NoError(t, err)
		require.True(t, alreadyInserted)
	})
}

func TestDequeueEmptyQueue(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()

		_, err := q.Select(ctx)
		require.Error(t, err)
		require.True(t, storage.ErrEmptyQueue.Has(err), "error should of class EmptyQueue")
	})
}

func TestSequential(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()

		const N = 20
		var addSegs []*internalpb.InjuredSegment
		for i := 0; i < N; i++ {
			seg := &internalpb.InjuredSegment{
				Path:       []byte(strconv.Itoa(i)),
				LostPieces: []int32{int32(i)},
			}
			alreadyInserted, err := q.Insert(ctx, seg, 10)
			require.NoError(t, err)
			require.False(t, alreadyInserted)
			addSegs = append(addSegs, seg)
		}

		list, err := q.SelectN(ctx, N)
		require.NoError(t, err)
		require.Len(t, list, N)

		sort.SliceStable(list, func(i, j int) bool { return list[i].LostPieces[0] < list[j].LostPieces[0] })

		for i := 0; i < N; i++ {
			require.True(t, pb.Equal(addSegs[i], &list[i]))
		}

		// TODO: fix out of order issue
		for i := 0; i < N; i++ {
			s, err := q.Select(ctx)
			require.NoError(t, err)
			err = q.Delete(ctx, s)
			require.NoError(t, err)
			expected := s.LostPieces[0]
			require.True(t, pb.Equal(addSegs[expected], s))
		}
	})
}

func TestParallel(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()
		const N = 20
		entries := make(chan *internalpb.InjuredSegment, N)

		var inserts errs2.Group
		// Add to queue concurrently
		for i := 0; i < N; i++ {
			i := i
			inserts.Go(func() error {
				_, err := q.Insert(ctx, &internalpb.InjuredSegment{
					Path:       []byte(strconv.Itoa(i)),
					LostPieces: []int32{int32(i)},
				}, 10)
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

		var items []*internalpb.InjuredSegment
		for segment := range entries {
			items = append(items, segment)
		}

		sort.Slice(items, func(i, k int) bool {
			return items[i].LostPieces[0] < items[k].LostPieces[0]
		})

		// check if the enqueued and dequeued elements match
		for i := 0; i < N; i++ {
			require.Equal(t, items[i].LostPieces[0], int32(i))
		}
	})
}

func TestClean(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		q := db.RepairQueue()

		seg1 := &internalpb.InjuredSegment{
			Path:       []byte("seg1"),
			LostPieces: []int32{int32(1), int32(3)},
		}
		seg2 := &internalpb.InjuredSegment{
			Path:       []byte("seg2"),
			LostPieces: []int32{int32(1), int32(3)},
		}
		seg3 := &internalpb.InjuredSegment{
			Path:       []byte("seg3"),
			LostPieces: []int32{int32(1), int32(3)},
		}

		timeBeforeInsert1 := time.Now()

		numHealthy := 10
		_, err := q.Insert(ctx, seg1, numHealthy)
		require.NoError(t, err)

		_, err = q.Insert(ctx, seg2, numHealthy)
		require.NoError(t, err)

		_, err = q.Insert(ctx, seg3, numHealthy)
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
		_, err = q.Insert(ctx, seg2, numHealthy)
		require.NoError(t, err)

		// seg3 has a lower health
		_, err = q.Insert(ctx, seg3, numHealthy-1)
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
