// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobqueue_test

import (
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/jobqueue"
)

// mustUUID generates a UUID. We use this in favor of testrand.UUID() to avoid
// collisions when we generate a large number of UUIDs (testrand UUIDs are
// seeded with only 32 bits of entropy and are likely to cause collisions when
// more than a few thousand are generated).
func mustUUID() uuid.UUID {
	u, err := uuid.New()
	if err != nil {
		panic(err)
	}
	return u
}

func TestQueue(t *testing.T) {
	queue, err := jobqueue.NewQueue(zaptest.NewLogger(t), time.Hour, 100, 10)
	require.NoError(t, err)

	timeIncrement := time.Duration(0)
	timeFunc := func() time.Time {
		return time.Now().Add(timeIncrement)
	}
	queue.Now = timeFunc

	// create and insert stream IDs
	const numStreams = 100
	jobs := make([]jobq.RepairJob, numStreams)
	for i := range jobs {
		jobs[i].ID.StreamID = mustUUID()
		jobs[i].ID.Position = rand.Uint64()
		jobs[i].Priority = float64(i)
		jobs[i].InsertedAt = uint64(time.Now().Unix())
		jobs[i].Placement = 42
	}
	for _, job := range jobs {
		wasNew := queue.Insert(job)
		require.True(t, wasNew)
	}

	// update one in the middle
	timeIncrement += time.Second
	const specialStream = 25
	const specialPriority = 500.0
	specialJob := jobs[specialStream]
	specialJob.Priority = specialPriority
	wasNew := queue.Insert(specialJob)
	require.False(t, wasNew)

	// see if that one got sorted to first
	gotJob := queue.Pop()
	require.Equal(t, specialPriority, gotJob.Priority)
	require.Equal(t, specialJob.ID, gotJob.ID)
	require.Greater(t, gotJob.UpdatedAt, gotJob.InsertedAt)
	specialJob.UpdatedAt = gotJob.UpdatedAt
	require.Equal(t, specialJob, gotJob)

	// pop the rest (expect them in reverse order of how we inserted)
	for i := numStreams - 1; i >= 0; i-- {
		if i == specialStream {
			// already popped this one
			continue
		}
		gotJob = queue.Pop()
		require.NotZero(t, gotJob.ID.StreamID)
		require.Equal(t, jobs[i].ID.StreamID, gotJob.ID.StreamID, i)
		require.Equal(t, jobs[i].ID.Position, gotJob.ID.Position, i)
		require.Equal(t, float64(i), jobs[i].Priority, i)
	}

	// pop an empty queue
	gotJob = queue.Pop()
	require.Zero(t, gotJob.ID.StreamID)
	require.Zero(t, gotJob.ID.Position)
	require.Zero(t, gotJob.Priority)
}

func TestQueueOfOneElement(t *testing.T) {
	queue, err := jobqueue.NewQueue(zaptest.NewLogger(t), time.Hour, 100, 10)
	require.NoError(t, err)

	job := jobq.RepairJob{
		ID:         jobq.SegmentIdentifier{StreamID: mustUUID(), Position: rand.Uint64()},
		Priority:   1.0,
		InsertedAt: uint64(time.Now().Unix()),
		Placement:  42,
	}
	wasNew := queue.Insert(job)
	require.True(t, wasNew)

	gotJob := queue.Pop()
	require.Equal(t, job.ID.StreamID, gotJob.ID.StreamID)
	require.Equal(t, job.ID.Position, gotJob.ID.Position)
	require.Equal(t, job.Priority, gotJob.Priority)

	gotJob = queue.Pop()
	require.Zero(t, gotJob.ID.StreamID)
	require.Zero(t, gotJob.ID.Position)
	require.Zero(t, gotJob.Priority)
}

func TestMemoryManagement(t *testing.T) {
	queue, err := jobqueue.NewQueue(zaptest.NewLogger(t), time.Hour, 10, 5)
	require.NoError(t, err)

	// grow the queue to 100 elements (which should entail several resizes)
	// and then shrink it back down to 0 (which should entail several markUnused
	// calls) before destroying it.
	jobs := make([]jobq.RepairJob, 1000)
	for i := range jobs {
		jobs[i] = jobq.RepairJob{
			ID:         jobq.SegmentIdentifier{StreamID: mustUUID(), Position: rand.Uint64()},
			Priority:   rand.Float64(),
			InsertedAt: uint64(time.Now().Unix()),
			Placement:  42,
		}
		queue.Insert(jobs[i])
		inRepair, _ := queue.Len()
		require.Equal(t, int64(i+1), inRepair)
	}
	// sort the inserted jobs and make sure they come out intact and in the
	// right order (ensuring memory was not overwritten or otherwise corrupted).
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].Priority > jobs[j].Priority
	})
	for i := range jobs {
		gotJob := queue.Pop()
		require.Equal(t, jobs[i].ID, gotJob.ID)
		inRepair, _ := queue.Len()
		require.Equal(t, int64(len(jobs)-i-1), inRepair)
	}
	queue.Destroy()
}

func BenchmarkQueue(b *testing.B) {
	queue, err := jobqueue.NewQueue(zaptest.NewLogger(b), time.Hour, 100, 10)
	require.NoError(b, err)
	jobs := make([]jobq.RepairJob, b.N)
	for i := range jobs {
		jobs[i].ID.StreamID = mustUUID()
		jobs[i].Priority = rand.Float64()
	}
	// insert initially
	for i := 0; i < b.N; i++ {
		wasNew := queue.Insert(jobs[i])
		require.True(b, wasNew)
	}
	// update all to different values
	for i := 0; i < b.N; i++ {
		jobs[i].Priority = rand.Float64()
		wasNew := queue.Insert(jobs[i])
		require.False(b, wasNew)
	}
	// pop all
	for i := 0; i < b.N; i++ {
		queue.Pop()
	}
	// ensure empty
	repairLen, retryLen := queue.Len()
	require.Zero(b, repairLen)
	require.Zero(b, retryLen)
}

func BenchmarkQueueInsertionOnly(b *testing.B) {
	queue, err := jobqueue.NewQueue(zaptest.NewLogger(b), time.Hour, 100, 10)
	require.NoError(b, err)
	jobs := make([]jobq.RepairJob, b.N)
	for i := range jobs {
		jobs[i].ID.StreamID = mustUUID()
		jobs[i].Priority = rand.Float64()
	}
	for i := 0; i < b.N; i++ {
		queue.Insert(jobs[i])
	}
}

func BenchmarkQueueUpdateOnly(b *testing.B) {
	queue, err := jobqueue.NewQueue(zaptest.NewLogger(b), time.Hour, b.N, 10)
	require.NoError(b, err)
	jobs := make([]jobq.RepairJob, b.N)
	for i := range jobs {
		jobs[i].ID.StreamID = mustUUID()
		jobs[i].Priority = rand.Float64()
	}
	for i := 0; i < b.N; i++ {
		queue.Insert(jobs[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jobs[i].Priority = rand.Float64()
		queue.Insert(jobs[i])
	}
}

func BenchmarkQueuePopOnly(b *testing.B) {
	queue, err := jobqueue.NewQueue(zaptest.NewLogger(b), time.Hour, b.N, 10)
	require.NoError(b, err)
	jobs := make([]jobq.RepairJob, b.N)
	for i := range jobs {
		jobs[i].ID.StreamID = mustUUID()
		jobs[i].Priority = rand.Float64()
	}
	for i := 0; i < b.N; i++ {
		queue.Insert(jobs[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.Pop()
	}
	// ensure empty
	repairLen, retryLen := queue.Len()
	require.Zero(b, repairLen)
	require.Zero(b, retryLen)
}
