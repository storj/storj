// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue_test

import (
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage/redis"
	"storj.io/storj/storage/redis/redisserver"
	"storj.io/storj/storage/testqueue"
)

func TestEnqueueDequeue(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		q := db.RepairQueue()

		seg := &pb.InjuredSegment{
			Path:       "abc",
			LostPieces: []int32{int32(1), int32(3)},
		}
		err := q.Enqueue(ctx, seg)
		assert.NoError(t, err)

		s, err := q.Dequeue(ctx)
		assert.NoError(t, err)
		assert.True(t, pb.Equal(&s, seg))
	})
}

func TestDequeueEmptyQueue(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		q := db.RepairQueue()

		s, err := q.Dequeue(ctx)
		assert.Error(t, err)
		assert.Equal(t, pb.InjuredSegment{}, s)
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
			err := q.Enqueue(ctx, seg)
			assert.NoError(t, err)
			addSegs = append(addSegs, seg)
		}

		list, err := q.Peekqueue(ctx, 100)
		assert.NoError(t, err)
		for i := 0; i < N; i++ {
			assert.True(t, pb.Equal(addSegs[i], &list[i]))
		}

		// TODO: fix out of order issue
		for i := 0; i < N; i++ {
			dequeued, err := q.Dequeue(ctx)
			assert.NoError(t, err)
			expected := dequeued.LostPieces[0]
			assert.True(t, pb.Equal(addSegs[expected], &dequeued))
		}
	})
}

func TestParallel(t *testing.T) {
	t.Skip("logic is broken on database side")

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		q := db.RepairQueue()
		const N = 100
		errs := make(chan error, N*2)
		entries := make(chan *pb.InjuredSegment, N*2)
		var wg sync.WaitGroup

		wg.Add(N)
		// Add to queue concurrently
		for i := 0; i < N; i++ {
			go func(i int) {
				defer wg.Done()
				err := q.Enqueue(ctx, &pb.InjuredSegment{
					Path:       strconv.Itoa(i),
					LostPieces: []int32{int32(i)},
				})
				if err != nil {
					errs <- err
				}
			}(i)

		}
		wg.Wait()

		wg.Add(N)
		// Remove from queue concurrently
		for i := 0; i < N; i++ {
			go func(i int) {
				defer wg.Done()
				segment, err := q.Dequeue(ctx)
				if err != nil {
					errs <- err
				}
				entries <- &segment
			}(i)
		}
		wg.Wait()
		close(errs)
		close(entries)

		for err := range errs {
			t.Error(err)
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
			assert.Equal(t, items[i].LostPieces[0], int32(i))
		}
	})
}

func BenchmarkRedisSequential(b *testing.B) {
	addr, cleanup, err := redisserver.Start()
	defer cleanup()
	assert.NoError(b, err)
	client, err := redis.NewQueue(addr, "", 1)
	assert.NoError(b, err)
	q := queue.NewQueue(client)
	benchmarkSequential(b, q)
}

func BenchmarkTeststoreSequential(b *testing.B) {
	q := queue.NewQueue(testqueue.New())
	benchmarkSequential(b, q)
}

func benchmarkSequential(b *testing.B, q queue.RepairQueue) {
	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		const N = 100
		var addSegs []*pb.InjuredSegment
		for i := 0; i < N; i++ {
			seg := &pb.InjuredSegment{
				Path:       strconv.Itoa(i),
				LostPieces: []int32{int32(i)},
			}
			err := q.Enqueue(ctx, seg)
			assert.NoError(b, err)
			addSegs = append(addSegs, seg)
		}
		for i := 0; i < N; i++ {
			dqSeg, err := q.Dequeue(ctx)
			assert.NoError(b, err)
			assert.True(b, pb.Equal(addSegs[i], &dqSeg))
		}
	}
}

func BenchmarkRedisParallel(b *testing.B) {
	addr, cleanup, err := redisserver.Start()
	defer cleanup()
	assert.NoError(b, err)
	client, err := redis.NewQueue(addr, "", 1)
	assert.NoError(b, err)
	q := queue.NewQueue(client)
	benchmarkParallel(b, q)
}

func BenchmarkTeststoreParallel(b *testing.B) {
	q := queue.NewQueue(testqueue.New())
	benchmarkParallel(b, q)
}

func benchmarkParallel(b *testing.B, q queue.RepairQueue) {
	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		const N = 100
		errs := make(chan error, N*2)
		entries := make(chan *pb.InjuredSegment, N*2)
		var wg sync.WaitGroup

		wg.Add(N)
		// Add to queue concurrently
		for i := 0; i < N; i++ {
			go func(i int) {
				defer wg.Done()
				err := q.Enqueue(ctx, &pb.InjuredSegment{
					Path:       strconv.Itoa(i),
					LostPieces: []int32{int32(i)},
				})
				if err != nil {
					errs <- err
				}
			}(i)

		}
		wg.Wait()
		wg.Add(N)
		// Remove from queue concurrently
		for i := 0; i < N; i++ {
			go func(i int) {
				defer wg.Done()
				segment, err := q.Dequeue(ctx)
				if err != nil {
					errs <- err
				}
				entries <- &segment
			}(i)
		}
		wg.Wait()
		close(errs)
		close(entries)

		for err := range errs {
			b.Error(err)
		}

		var items []*pb.InjuredSegment
		for segment := range entries {
			items = append(items, segment)
		}

		sort.Slice(items, func(i, k int) bool { return items[i].LostPieces[0] < items[k].LostPieces[0] })
		// check if the enqueued and dequeued elements match
		for i := 0; i < N; i++ {
			assert.Equal(b, items[i].LostPieces[0], int32(i))
		}
	}
}
