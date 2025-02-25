// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build goexperiment.synctest && go1.24

package jobqueue_test

import (
	"math/rand"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testrand"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/jobqueue"
)

func TestJobqueueRetry(t *testing.T) {
	synctest.Run(func() {
		const retryTime = time.Hour
		queue, err := jobqueue.NewQueue(zaptest.NewLogger(t), retryTime, 100, 10)
		require.NoError(t, err)
		startTime := time.Now()

		queue.Start()
		defer queue.Stop()

		// create and insert stream IDs
		const numStreams = 10
		jobs := make([]jobq.RepairJob, numStreams)
		highestPrio := -1
		for i := range jobs {
			jobs[i].ID.StreamID = testrand.UUID()
			jobs[i].ID.Position = rand.Uint64()
			jobs[i].Priority = rand.Float64()
			jobs[i].Placement = 42
			jobs[i].NumMissing = uint16(rand.Uint32() % 80)
			jobs[i].NumOutOfPlacement = uint16(rand.Uint32() % 80)
			if highestPrio == -1 || jobs[i].Priority > jobs[highestPrio].Priority {
				highestPrio = i
			}
		}
		for _, job := range jobs {
			queue.Insert(job)
		}

		// pull out highest priority
		firstJob := queue.Pop()
		require.NotZero(t, firstJob.ID.StreamID)
		jobs[highestPrio].InsertedAt = firstJob.InsertedAt
		jobs[highestPrio].UpdatedAt = firstJob.UpdatedAt
		require.Equal(t, jobs[highestPrio], firstJob)

		// call the repair failed; put it back in
		firstJob.LastAttemptedAt = uint64(time.Now().Unix())
		queue.Insert(firstJob)

		// pull out the next few highest priorities (none of them should match the
		// first because the first one should be in the retry queue)
		for range 4 {
			nextJob := queue.Pop()
			require.NotZero(t, nextJob.ID.StreamID)
			require.NotEqual(t, firstJob.ID, nextJob.ID)
			time.Sleep(time.Second)
		}

		// add another retry job that will be eligible for retry sooner than
		// firstJob (to make sure the funnel goroutine adjusts its sleep)
		secondRetryJob := jobq.RepairJob{
			ID:              jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: rand.Uint64()},
			Priority:        1.1,
			InsertedAt:      uint64(startTime.Add(-2 * retryTime).Unix()),
			Placement:       42,
			LastAttemptedAt: uint64(startTime.Add(-retryTime / 2).Unix()),
		}
		queue.Insert(secondRetryJob)

		// pull out next priority job; should still not be firstJob or secondRetryJob
		nextJob := queue.Pop()
		require.NotZero(t, nextJob.ID.StreamID)
		require.NotEqual(t, firstJob.ID, nextJob.ID)
		require.NotEqual(t, secondRetryJob.ID, nextJob.ID)

		time.Sleep(retryTime/2 + 1)

		// now secondRetryJob should be eligible for retry and it should have the highest priority
		nextJob = queue.Pop()
		require.NotZero(t, nextJob.ID.StreamID)
		require.Equal(t, secondRetryJob.ID, nextJob.ID)

		// pop the rest of the original jobs
		for range 4 {
			nextJob = queue.Pop()
			require.NotZero(t, nextJob.ID.StreamID)
			require.NotEqual(t, firstJob.ID, nextJob.ID)
			require.NotEqual(t, secondRetryJob.ID, nextJob.ID)
		}

		// wait until firstJob is eligible for retry and is yielded
		require.True(t, time.Now().Before(firstJob.LastAttemptedAtTime().Add(retryTime)))

		for {
			nextJob = queue.Pop()
			if nextJob.ID.StreamID.IsZero() {
				time.Sleep(retryTime / 30) // exact value not important
				continue
			}
			require.Equal(t, firstJob.ID, nextJob.ID)
			break
		}
	})
}
