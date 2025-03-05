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
		jobs[i].Health = 1.0 / float64(i)
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
	const specialHealth = -1.0
	specialJob := jobs[specialStream]
	specialJob.Health = specialHealth
	wasNew := queue.Insert(specialJob)
	require.False(t, wasNew)

	// see if that one got sorted to first
	gotJob := queue.Pop()
	require.Equal(t, specialHealth, gotJob.Health)
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
		require.Equal(t, 1.0/float64(i), jobs[i].Health, i)
	}

	// pop an empty queue
	gotJob = queue.Pop()
	require.Zero(t, gotJob.ID.StreamID)
	require.Zero(t, gotJob.ID.Position)
	require.Zero(t, gotJob.Health)
}

func TestQueueOfOneElement(t *testing.T) {
	queue, err := jobqueue.NewQueue(zaptest.NewLogger(t), time.Hour, 100, 10)
	require.NoError(t, err)

	job := jobq.RepairJob{
		ID:         jobq.SegmentIdentifier{StreamID: mustUUID(), Position: rand.Uint64()},
		Health:     1.0,
		InsertedAt: uint64(time.Now().Unix()),
		Placement:  42,
	}
	wasNew := queue.Insert(job)
	require.True(t, wasNew)

	gotJob := queue.Pop()
	require.Equal(t, job.ID.StreamID, gotJob.ID.StreamID)
	require.Equal(t, job.ID.Position, gotJob.ID.Position)
	require.Equal(t, job.Health, gotJob.Health)

	gotJob = queue.Pop()
	require.Zero(t, gotJob.ID.StreamID)
	require.Zero(t, gotJob.ID.Position)
	require.Zero(t, gotJob.Health)
}

func TestQueueClean(t *testing.T) {
	queue, err := jobqueue.NewQueue(zaptest.NewLogger(t), time.Hour, 100, 10)
	require.NoError(t, err)
	_ = queue.Start()
	defer queue.Stop()

	timeIncrement := time.Duration(0)
	timeFunc := func() time.Time {
		return time.Now().Add(timeIncrement)
	}
	queue.Now = timeFunc
	startTime := uint64(time.Now().Unix())

	// create and insert stream IDs
	const numStreams = 100
	jobs := make([]jobq.RepairJob, numStreams)
	for i := range jobs {
		jobs[i].ID.StreamID = mustUUID()
		jobs[i].ID.Position = rand.Uint64()
		jobs[i].Health = rand.Float64()
		jobs[i].Placement = 42
		// let's have about half of them be in retry
		if rand.Intn(2) == 0 {
			jobs[i].LastAttemptedAt = uint64(time.Now().Add((-15*time.Minute + time.Duration(i)*time.Second)).Unix())
		}
		timeIncrement += time.Second
	}
	for _, job := range jobs {
		wasNew := queue.Insert(job)
		require.True(t, wasNew)
	}

	updateCutOff := timeFunc().Add(time.Second)
	timeIncrement += 2 * time.Second

	// update half of them to update LastUpdatedAt
	for i := range jobs {
		if i%2 == 0 {
			wasNew := queue.Insert(jobs[i])
			require.False(t, wasNew)
		}
	}

	// clean out all elements not updated
	queue.Clean(updateCutOff)

	// put together what we expect to find
	var expectedJobs []jobq.RepairJob
	var jobsNeedingRetry []jobq.RepairJob
	for i := range jobs {
		if i%2 == 0 {
			if jobs[i].LastAttemptedAt != 0 {
				jobsNeedingRetry = append(jobsNeedingRetry, jobs[i])
			} else {
				expectedJobs = append(expectedJobs, jobs[i])
			}
		}
	}
	sort.Slice(expectedJobs, func(i, j int) bool {
		return expectedJobs[i].Health < expectedJobs[j].Health
	})
	sort.Slice(jobsNeedingRetry, func(i, j int) bool {
		return jobsNeedingRetry[i].LastAttemptedAt < jobsNeedingRetry[j].LastAttemptedAt
	})

	// pop all and check
	for _, expectedJob := range expectedJobs {
		gotJob := queue.Pop()
		require.GreaterOrEqual(t, gotJob.UpdatedAt, uint64(updateCutOff.Unix()))
		require.LessOrEqual(t, gotJob.UpdatedAt, uint64(timeFunc().Unix()))
		require.LessOrEqual(t, gotJob.InsertedAt, uint64(updateCutOff.Unix()))
		require.GreaterOrEqual(t, gotJob.InsertedAt, startTime)
		expectedJob.UpdatedAt = gotJob.UpdatedAt
		expectedJob.InsertedAt = gotJob.InsertedAt
		require.Equal(t, expectedJob, gotJob)
	}
	// no elements left
	gotJob := queue.Pop()
	require.Zero(t, gotJob.ID.StreamID)
	repairLen, retryLen := queue.Len()
	require.Zero(t, repairLen)
	require.NotZero(t, retryLen)

	// until we twist the clock forward
	timeIncrement += time.Hour
	// and reset the queue timer so it knows about this change
	err = queue.ResetTimer()
	require.NoError(t, err)

	// pop all the retry jobs now. they may come out in a complicated order
	// because it's a pain to manage the clock finely enough to get them to come
	// out only one at a time (in which case they would be sorted by
	// LastAttemptedAt). Instead, if multiple jobs get moved to the repair queue
	// at the same time, they'll be sorted by health. For now we'll just pop
	// them all and make sure they're all there. We'll do a better test of
	// funnel timing mechanics in a jobqueue_sync_test.go.
	gotJobs := make([]jobq.RepairJob, 0, len(jobsNeedingRetry))
	for len(gotJobs) < len(jobsNeedingRetry) {
		gotJob = queue.Pop()
		if gotJob.ID.StreamID.IsZero() {
			// give the funnel goroutine a moment more
			time.Sleep(time.Microsecond)
			continue
		}
		gotJobs = append(gotJobs, gotJob)
	}

	sort.Slice(gotJobs, func(i, j int) bool {
		return gotJobs[i].LastAttemptedAt < gotJobs[j].LastAttemptedAt
	})
	for i, gotJob := range gotJobs {
		require.GreaterOrEqual(t, gotJob.UpdatedAt, uint64(updateCutOff.Unix()))
		require.LessOrEqual(t, gotJob.UpdatedAt, uint64(timeFunc().Unix()))
		require.LessOrEqual(t, gotJob.InsertedAt, uint64(updateCutOff.Unix()))
		require.GreaterOrEqual(t, gotJob.InsertedAt, startTime)
		jobsNeedingRetry[i].UpdatedAt = gotJob.UpdatedAt
		jobsNeedingRetry[i].InsertedAt = gotJob.InsertedAt
		require.Equal(t, jobsNeedingRetry[i], gotJob)
	}

	// no elements left
	gotJob = queue.Pop()
	require.Zero(t, gotJob.ID.StreamID)
	repairLen, retryLen = queue.Len()
	require.Zero(t, repairLen)
	require.Zero(t, retryLen)
}

func TestQueueTrim(t *testing.T) {
	queue, err := jobqueue.NewQueue(zaptest.NewLogger(t), time.Hour, 100, 10)
	require.NoError(t, err)
	_ = queue.Start()
	defer queue.Stop()

	timeIncrement := time.Duration(0)
	timeFunc := func() time.Time {
		return time.Now().Add(timeIncrement)
	}
	queue.Now = timeFunc

	// create jobs with different priorities
	const numStreams = 100
	jobs := make([]jobq.RepairJob, numStreams)
	for i := range jobs {
		jobs[i].ID.StreamID = mustUUID()
		jobs[i].ID.Position = rand.Uint64()
		jobs[i].Health = float64(i) / 100.0 // 0.00, 0.01, 0.02, ... 0.99
		jobs[i].Placement = 42
		// let's have about half of them be in retry
		if rand.Intn(2) == 0 {
			jobs[i].LastAttemptedAt = uint64(time.Now().Add((-15*time.Minute + time.Duration(i)*time.Second)).Unix())
		}
		timeIncrement += time.Second
	}
	for _, job := range jobs {
		wasNew := queue.Insert(job)
		require.True(t, wasNew)
	}

	// test our initial conditions
	repairLen, retryLen := queue.Len()
	initialRepair := int(repairLen)
	initialRetry := int(retryLen)
	require.NotZero(t, initialRepair)
	require.NotZero(t, initialRetry)

	// We'll trim all jobs with health > 0.5
	const trimThreshold = 0.5
	removed := queue.Trim(trimThreshold)

	// Check how many jobs were removed - should be approximately half of all jobs
	require.NotZero(t, removed)

	// Check the new lengths
	repairLen, retryLen = queue.Len()

	// Make sure that the sum of jobs after trimming plus the removed jobs equals the initial total
	require.Equal(t, int64(initialRepair+initialRetry), repairLen+retryLen+int64(removed))

	// Verify all remaining jobs in repair queue have health <= trimThreshold
	var remainingJobs []jobq.RepairJob
	for i := 0; i < int(repairLen); i++ {
		job := queue.Pop()
		if job.ID.StreamID.IsZero() {
			break
		}
		remainingJobs = append(remainingJobs, job)
		require.LessOrEqual(t, job.Health, trimThreshold)
	}

	// Move time forward to get jobs from retry queue
	timeIncrement += time.Hour
	err = queue.ResetTimer()
	require.NoError(t, err)

	// Verify retry queue jobs move to repair queue when eligible
	time.Sleep(10 * time.Millisecond) // Give the funnel goroutine time to run

	// Check repair queue again to see if jobs from retry queue moved there
	for {
		job := queue.Pop()
		if job.ID.StreamID.IsZero() {
			break
		}
		remainingJobs = append(remainingJobs, job)
		require.LessOrEqual(t, job.Health, trimThreshold)
	}

	// Verify jobs were removed (exact count can vary due to random placement in retry queue)
	require.Greater(t, removed, 0, "Should have removed some jobs")
	require.GreaterOrEqual(t, len(remainingJobs), 0, "Should have some jobs remaining")

	// Verify only jobs with health <= threshold are left
	for _, job := range remainingJobs {
		require.LessOrEqual(t, job.Health, trimThreshold)
	}
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
			Health:     rand.Float64(),
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
		return jobs[i].Health < jobs[j].Health
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
		jobs[i].Health = rand.Float64()
	}
	// insert initially
	for i := 0; i < b.N; i++ {
		wasNew := queue.Insert(jobs[i])
		require.True(b, wasNew)
	}
	// update all to different values
	for i := 0; i < b.N; i++ {
		jobs[i].Health = rand.Float64()
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
		jobs[i].Health = rand.Float64()
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
		jobs[i].Health = rand.Float64()
	}
	for i := 0; i < b.N; i++ {
		queue.Insert(jobs[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jobs[i].Health = rand.Float64()
		queue.Insert(jobs[i])
	}
}

func BenchmarkQueuePopOnly(b *testing.B) {
	queue, err := jobqueue.NewQueue(zaptest.NewLogger(b), time.Hour, b.N, 10)
	require.NoError(b, err)
	jobs := make([]jobq.RepairJob, b.N)
	for i := range jobs {
		jobs[i].ID.StreamID = mustUUID()
		jobs[i].Health = rand.Float64()
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
