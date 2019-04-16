// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue_test

import (
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestInsertDequeue(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		q := db.RepairQueue()

		seg := &pb.InjuredSegment{
			Path:       "abc",
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

func TestDequeueEmptyQueue(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		q := db.RepairQueue()

		s, err := q.Select(ctx)
		require.Error(t, err)
		require.Nil(t, s)
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
				Path:       strconv.Itoa(i),
				LostPieces: []int32{int32(i)},
			}
			err := q.Insert(ctx, seg)
			require.NoError(t, err)
			addSegs = append(addSegs, seg)
		}

		list, err := q.SelectN(ctx, 100)
		require.NoError(t, err)
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
		errs := make(chan error, N*2)
		entries := make(chan *pb.InjuredSegment, N)
		var wg sync.WaitGroup
		wg.Add(N)
		// Add to queue concurrently
		for i := 0; i < N; i++ {
			go func(i int) {
				defer wg.Done()
				err := q.Insert(ctx, &pb.InjuredSegment{
					Path:       strconv.Itoa(i),
					LostPieces: []int32{int32(i)},
				})
				if err != nil {
					errs <- err
				}
			}(i)
		}
		wg.Wait()

		if len(errs) > 0 {
			for err := range errs {
				t.Error(err)
			}

			t.Fatal("unexpected queue.Insert errors")
		}

		wg.Add(N)
		// Remove from queue concurrently
		for i := 0; i < N; i++ {
			go func(i int) {
				defer wg.Done()
				s, err := q.Select(ctx)
				if err != nil {
					errs <- err
				}

				err = q.Delete(ctx, s)
				if err != nil {
					errs <- err
				}

				entries <- s
			}(i)
		}
		wg.Wait()
		close(errs)
		close(entries)

		if len(errs) > 0 {
			for err := range errs {
				t.Error(err)
			}

			t.Fatal("unexpected queue.Select/Delete errors")
		}

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

// func BenchmarkRedisSequential(b *testing.B) {
// 	addr, cleanup, err := redisserver.Start()
// 	defer cleanup()
// 	require.NoError(b, err)
// 	client, err := redis.NewQueue(addr, "", 1)
// 	require.NoError(b, err)
// 	q := queue.NewQueue(client)
// 	benchmarkSequential(b, q)
// }

// func BenchmarkTeststoreSequential(b *testing.B) {
// 	q := queue.NewQueue(testqueue.New())
// 	benchmarkSequential(b, q)
// }

// func benchmarkSequential(b *testing.B, q queue.RepairQueue) {
// 	ctx := testcontext.New(b)
// 	defer ctx.Cleanup()

// 	b.ResetTimer()
// 	for n := 0; n < b.N; n++ {
// 		const N = 100
// 		var addSegs []*pb.InjuredSegment
// 		for i := 0; i < N; i++ {
// 			seg := &pb.InjuredSegment{
// 				Path:       strconv.Itoa(i),
// 				LostPieces: []int32{int32(i)},
// 			}
// 			err := q.Insert(ctx, seg)
// 			require.NoError(b, err)
// 			addSegs = append(addSegs, seg)
// 		}
// 		for i := 0; i < N; i++ {
// 			s, err := q.Select(ctx)
// 			require.NoError(b, err)
// 			err = q.Delete(ctx, s)
// 			require.NoError(b, err)
// 			require.True(b, pb.Equal(addSegs[i], s))
// 		}
// 	}
// }

// func BenchmarkRedisParallel(b *testing.B) {
// 	addr, cleanup, err := redisserver.Start()
// 	defer cleanup()
// 	require.NoError(b, err)
// 	client, err := redis.NewQueue(addr, "", 1)
// 	require.NoError(b, err)
// 	q := queue.NewQueue(client)
// 	benchmarkParallel(b, q)
// }

// func BenchmarkTeststoreParallel(b *testing.B) {
// 	q := queue.NewQueue(testqueue.New())
// 	benchmarkParallel(b, q)
// }

// func benchmarkParallel(b *testing.B, q queue.RepairQueue) {
// 	ctx := testcontext.New(b)
// 	defer ctx.Cleanup()

// 	b.ResetTimer()
// 	for n := 0; n < b.N; n++ {
// 		const N = 100
// 		errs := make(chan error, N*2)
// 		entries := make(chan *pb.InjuredSegment, N*2)
// 		var wg sync.WaitGroup

// 		wg.Add(N)
// 		// Add to queue concurrently
// 		for i := 0; i < N; i++ {
// 			go func(i int) {
// 				defer wg.Done()
// 				err := q.Insert(ctx, &pb.InjuredSegment{
// 					Path:       strconv.Itoa(i),
// 					LostPieces: []int32{int32(i)},
// 				})
// 				if err != nil {
// 					errs <- err
// 				}
// 			}(i)

// 		}
// 		wg.Wait()
// 		wg.Add(N)
// 		// Remove from queue concurrently
// 		for i := 0; i < N; i++ {
// 			go func(i int) {
// 				defer wg.Done()
// 				s, err := q.Select(ctx)
// 				require.NoError(b, err)
// 				err = q.Delete(ctx, s)
// 				require.NoError(b, err)
// 				if err != nil {
// 					errs <- err
// 				}
// 				entries <- s
// 			}(i)
// 		}
// 		wg.Wait()
// 		close(errs)
// 		close(entries)

// 		for err := range errs {
// 			b.Error(err)
// 		}

// 		var items []*pb.InjuredSegment
// 		for segment := range entries {
// 			items = append(items, segment)
// 		}

// 		sort.Slice(items, func(i, k int) bool { return items[i].LostPieces[0] < items[k].LostPieces[0] })
// 		// check if the Insert and dequeued elements match
// 		for i := 0; i < N; i++ {
// 			require.Equal(b, items[i].LostPieces[0], int32(i))
// 		}
// 	}
// }
