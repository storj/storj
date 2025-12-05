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
		queue, err := jobqueue.NewQueue(zaptest.NewLogger(t), retryTime, 100, 0, 10)
		require.NoError(t, err)
		startTime := time.Now()

		err = queue.Start()
		require.NoError(t, err)
		defer queue.Stop()

		// create and insert stream IDs
		const numStreams = 10
		jobs := make([]jobq.RepairJob, numStreams)
		lowestHealth := -1
		for i := range jobs {
			jobs[i].ID.StreamID = testrand.UUID()
			jobs[i].ID.Position = rand.Uint64()
			jobs[i].Health = rand.Float64()
			jobs[i].Placement = 42
			jobs[i].NumMissing = uint16(rand.Uint32() % 80)
			jobs[i].NumOutOfPlacement = uint16(rand.Uint32() % 80)
			if lowestHealth == -1 || jobs[i].Health < jobs[lowestHealth].Health {
				lowestHealth = i
			}
		}
		for _, job := range jobs {
			queue.Insert(job)
		}

		// pull out lowest health
		firstJob, ok := queue.Pop()
		require.NotZero(t, firstJob.ID.StreamID)
		require.True(t, ok)
		jobs[lowestHealth].InsertedAt = firstJob.InsertedAt
		jobs[lowestHealth].UpdatedAt = firstJob.UpdatedAt
		require.Equal(t, jobs[lowestHealth], firstJob)

		// call the repair failed; put it back in
		firstJob.LastAttemptedAt = uint64(time.Now().Unix())
		queue.Insert(firstJob)

		// pull out the next few highest priorities (none of them should match the
		// first because the first one should be in the retry queue)
		for range 4 {
			nextJob, ok := queue.Pop()
			require.NotZero(t, nextJob.ID.StreamID)
			require.True(t, ok)
			require.NotEqual(t, firstJob.ID, nextJob.ID)
			time.Sleep(time.Second)
		}

		// add another retry job that will be eligible for retry sooner than
		// firstJob (to make sure the funnel goroutine adjusts its sleep)
		secondRetryJob := jobq.RepairJob{
			ID:              jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: rand.Uint64()},
			Health:          -1.0, // Negative health value to ensure highest priority
			InsertedAt:      uint64(startTime.Add(-2 * retryTime).Unix()),
			Placement:       42,
			LastAttemptedAt: uint64(startTime.Add(-retryTime / 2).Unix()),
		}
		queue.Insert(secondRetryJob)

		// pull out next job; should still not be firstJob or secondRetryJob
		nextJob, ok := queue.Pop()
		require.NotZero(t, nextJob.ID.StreamID)
		require.True(t, ok)
		require.NotEqual(t, firstJob.ID, nextJob.ID)
		require.NotEqual(t, secondRetryJob.ID, nextJob.ID)

		time.Sleep(retryTime/2 + 1)

		// now secondRetryJob should be eligible for retry and it should have the lowest health
		nextJob, ok = queue.Pop()
		require.True(t, ok)
		require.NotZero(t, nextJob.ID.StreamID)
		require.Equal(t, secondRetryJob.ID, nextJob.ID)

		// pop the rest of the original jobs
		for range 4 {
			nextJob, ok = queue.Pop()
			require.NotZero(t, nextJob.ID.StreamID)
			require.True(t, ok)
			require.NotEqual(t, firstJob.ID, nextJob.ID)
			require.NotEqual(t, secondRetryJob.ID, nextJob.ID)
		}

		// wait until firstJob is eligible for retry and is yielded
		require.True(t, time.Now().Before(firstJob.LastAttemptedAtTime().Add(retryTime)))

		for {
			nextJob, ok = queue.Pop()
			if !ok {
				time.Sleep(retryTime / 30) // exact value not important
				continue
			}
			require.Equal(t, firstJob.ID, nextJob.ID)
			break
		}
	})
}
