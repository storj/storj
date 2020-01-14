// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue_test

import (
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage"
)

func TestInsertSelect(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		q := db.RepairQueue()

		seg := &pb.InjuredSegment{
			Path:       []byte("abc"),
			LostPieces: []int32{int32(1), int32(3)},
		}
		err := q.Insert(ctx, seg)
		require.NoError(t, err)
		s, err := q.Select(ctx)
		require.NoError(t, err)
		err = q.Delete(ctx, s)
		require.NoError(t, err)
		require.True(t, pb.Equal(s, seg))
	})
}

func TestInsertDuplicate(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		q := db.RepairQueue()

		seg := &pb.InjuredSegment{
			Path:       []byte("abc"),
			LostPieces: []int32{int32(1), int32(3)},
		}
		err := q.Insert(ctx, seg)
		require.NoError(t, err)
		err = q.Insert(ctx, seg)
		require.NoError(t, err)
	})
}

func TestDequeueEmptyQueue(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		q := db.RepairQueue()

		_, err := q.Select(ctx)
		require.Error(t, err)
		require.True(t, storage.ErrEmptyQueue.Has(err), "error should of class EmptyQueue")
	})
}

func TestSequential(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		q := db.RepairQueue()

		const N = 100
		var addSegs []*pb.InjuredSegment
		for i := 0; i < N; i++ {
			seg := &pb.InjuredSegment{
				Path:       []byte(strconv.Itoa(i)),
				LostPieces: []int32{int32(i)},
			}
			err := q.Insert(ctx, seg)
			require.NoError(t, err)
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
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		q := db.RepairQueue()
		const N = 100
		entries := make(chan *pb.InjuredSegment, N)

		var inserts errs2.Group
		// Add to queue concurrently
		for i := 0; i < N; i++ {
			i := i
			inserts.Go(func() error {
				return q.Insert(ctx, &pb.InjuredSegment{
					Path:       []byte(strconv.Itoa(i)),
					LostPieces: []int32{int32(i)},
				})
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

		var items []*pb.InjuredSegment
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
