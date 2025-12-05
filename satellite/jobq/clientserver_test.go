// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobq_test

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/jobqtest"
)

func TestClientAndServer(t *testing.T) {
	jobqtest.WithServerAndClient(t, nil, func(ctx *testcontext.Context, srv *jobqtest.TestServer, cli *jobq.Client) {
		job := jobq.RepairJob{
			ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: 2},
			Health:    3.0,
			Placement: 42,
		}
		wasNew, err := cli.Push(ctx, job)
		require.NoError(t, err)
		require.True(t, wasNew)

		gotRepairLen, gotRetryLen, err := cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(1), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)

		gotJob, err := cli.Inspect(ctx, 42, job.ID.StreamID, job.ID.Position)
		require.NoError(t, err)
		require.NotZero(t, gotJob.InsertedAt)
		job.InsertedAt = gotJob.InsertedAt
		job.UpdatedAt = gotJob.UpdatedAt
		require.Equal(t, job, gotJob)

		gotJobs, err := cli.Pop(ctx, 1, nil, nil)
		require.NoError(t, err)
		require.Len(t, gotJobs, 1)
		require.Equal(t, job.ID.StreamID, gotJobs[0].ID.StreamID)
		require.Equal(t, uint64(2), gotJobs[0].ID.Position)
		require.Equal(t, 3.0, gotJobs[0].Health)
		require.Equal(t, uint16(42), gotJobs[0].Placement)

		gotRepairLen, gotRetryLen, err = cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(0), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)

		gotJobs, err = cli.Pop(ctx, 1, nil, nil)
		require.NoError(t, err)
		require.Len(t, gotJobs, 0)
	})
}

func TestClientServerPushBatch(t *testing.T) {
	jobqtest.WithServerAndClient(t, nil, func(ctx *testcontext.Context, srv *jobqtest.TestServer, cli *jobq.Client) {
		// Create multiple jobs
		jobs := []jobq.RepairJob{
			{
				ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: 2},
				Health:    4.0,
				Placement: 42,
			},
			{
				ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: 3},
				Health:    3.0,
				Placement: 42,
			},
		}

		// Push batch of jobs
		newJobs, err := cli.PushBatch(ctx, jobs)
		require.NoError(t, err)
		require.Len(t, newJobs, 2)
		require.True(t, newJobs[0])
		require.True(t, newJobs[1])

		// Verify length
		gotRepairLen, gotRetryLen, err := cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(2), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)

		// Pop jobs to verify they were inserted correctly
		gotJobs1, err := cli.Pop(ctx, 1, nil, nil)
		require.NoError(t, err)
		require.Len(t, gotJobs1, 1)
		gotJobs2, err := cli.Pop(ctx, 1, nil, nil)
		require.NoError(t, err)
		require.Len(t, gotJobs2, 1)

		// Lower health job should come first
		require.Equal(t, jobs[1].ID.StreamID, gotJobs1[0].ID.StreamID)
		require.Equal(t, jobs[0].ID.StreamID, gotJobs2[0].ID.StreamID)
	})
}

func TestClientServerPeek(t *testing.T) {
	jobqtest.WithServerAndClient(t, nil, func(ctx *testcontext.Context, srv *jobqtest.TestServer, cli *jobq.Client) {
		// Create and push a job
		job := jobq.RepairJob{
			ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: 2},
			Health:    3.0,
			Placement: 42,
		}
		wasNew, err := cli.Push(ctx, job)
		require.NoError(t, err)
		require.True(t, wasNew)

		// Peek the job
		peekedJobs, err := cli.Peek(ctx, 1, []storj.PlacementConstraint{42}, nil)
		require.NoError(t, err)
		require.Len(t, peekedJobs, 1)
		require.Equal(t, job.ID.StreamID, peekedJobs[0].ID.StreamID)
		require.Equal(t, job.ID.Position, peekedJobs[0].ID.Position)
		require.Equal(t, job.Health, peekedJobs[0].Health)

		// Verify the job is still in the queue after peeking
		gotRepairLen, gotRetryLen, err := cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(1), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)

		// Pop the job to ensure it's still there
		gotJobs, err := cli.Pop(ctx, 1, nil, nil)
		require.NoError(t, err)
		require.Len(t, gotJobs, 1)
		require.Equal(t, job.ID.StreamID, gotJobs[0].ID.StreamID)
		require.Equal(t, job.ID.Position, gotJobs[0].ID.Position)
	})
}

func TestClientServerTruncate(t *testing.T) {
	jobqtest.WithServerAndClient(t, nil, func(ctx *testcontext.Context, srv *jobqtest.TestServer, cli *jobq.Client) {
		// Create and push a few jobs
		for i := 0; i < 5; i++ {
			job := jobq.RepairJob{
				ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: uint64(i)},
				Health:    float64(i),
				Placement: 42,
			}
			wasNew, err := cli.Push(ctx, job)
			require.NoError(t, err)
			require.True(t, wasNew)
		}

		// Verify we have jobs in the queue
		gotRepairLen, gotRetryLen, err := cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(5), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)

		// Truncate the queue
		err = cli.Truncate(ctx, 42)
		require.NoError(t, err)

		// Verify the queue is empty after truncation
		gotRepairLen, gotRetryLen, err = cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(0), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)

		// Try to pop a job, should be empty
		gotJobs, err := cli.Pop(ctx, 1, nil, nil)
		require.NoError(t, err)
		require.Len(t, gotJobs, 0)
	})
}

func TestClientServerClean(t *testing.T) {
	jobqtest.WithServerAndClient(t, nil, func(ctx *testcontext.Context, srv *jobqtest.TestServer, cli *jobq.Client) {
		// Set up our time control
		now := time.Now()
		timeIncrement := time.Duration(0)
		timeFunc := func() time.Time {
			return now.Add(timeIncrement)
		}

		// make the queue for placement 42 get initialized now, so we can change
		// its time function
		_, err := srv.Jobq.QueueMap.GetQueue(42)
		require.NoError(t, err)
		srv.SetTimeFunc(timeFunc)

		// First batch of jobs with the current time
		for i := 0; i < 3; i++ {
			job := jobq.RepairJob{
				ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: uint64(i)},
				Health:    float64(i),
				Placement: 42,
			}
			_, err = cli.Push(ctx, job)
			require.NoError(t, err)
		}

		// Move time forward 2 hours for the second batch
		timeIncrement = 2 * time.Hour

		// Push some "fresh" jobs with the new time
		for i := 3; i < 5; i++ {
			job := jobq.RepairJob{
				ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: uint64(i)},
				Health:    float64(i),
				Placement: 42,
			}
			_, err = cli.Push(ctx, job)
			require.NoError(t, err)
		}

		// Verify we have all jobs in the queue
		gotRepairLen, gotRetryLen, err := cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(5), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)

		// Clean jobs updated before 1 hour ago from current time
		cutoffTime := timeFunc().Add(-1 * time.Hour)
		removedSegments, err := cli.Clean(ctx, 42, cutoffTime)
		require.NoError(t, err)
		require.Equal(t, int32(3), removedSegments)

		// Verify only fresh jobs remain
		gotRepairLen, gotRetryLen, err = cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(2), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)
	})
}

func TestClientServerTrim(t *testing.T) {
	jobqtest.WithServerAndClient(t, nil, func(ctx *testcontext.Context, srv *jobqtest.TestServer, cli *jobq.Client) {
		// Set up our time control
		now := time.Now()
		timeIncrement := time.Duration(0)
		timeFunc := func() time.Time {
			return now.Add(timeIncrement)
		}
		srv.SetTimeFunc(timeFunc)

		// Create and push jobs with different health values
		for i := 0; i < 5; i++ {
			job := jobq.RepairJob{
				ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: uint64(i)},
				Health:    1.0 / float64(i),
				Placement: 42,
			}
			_, err := cli.Push(ctx, job)
			require.NoError(t, err)

			// Small time increment to ensure distinct timestamps
			timeIncrement += time.Second
		}

		// Verify we have all jobs in the queue
		gotRepairLen, gotRetryLen, err := cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(5), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)

		// Trim jobs with health > 1/3
		removedSegments, err := cli.Trim(ctx, 42, 1.0/3)
		require.NoError(t, err)
		require.Equal(t, int32(3), removedSegments)

		// Verify only low-health jobs remain
		gotRepairLen, gotRetryLen, err = cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(2), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)

		// Pop jobs to verify their health values
		jobs1, err := cli.Pop(ctx, 1, nil, nil)
		require.NoError(t, err)
		require.Len(t, jobs1, 1)
		require.LessOrEqual(t, jobs1[0].Health, 1.0/3)

		jobs2, err := cli.Pop(ctx, 1, nil, nil)
		require.NoError(t, err)
		require.Len(t, jobs2, 1)
		require.LessOrEqual(t, jobs2[0].Health, 1.0/3)

		// Queue should be empty now
		jobs3, err := cli.Pop(ctx, 1, nil, nil)
		require.NoError(t, err)
		require.Len(t, jobs3, 0)
	})
}

func TestStat(t *testing.T) {
	jobqtest.WithServerAndClient(t, nil, func(ctx *testcontext.Context, server *jobqtest.TestServer, client *jobq.Client) {
		// Create and push a few jobs
		for i := 0; i < 5; i++ {
			job := jobq.RepairJob{
				ID:         jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: uint64(i)},
				Health:     float64(i),
				Placement:  42,
				InsertedAt: uint64(time.Now().Unix()),
			}
			_, err := client.Push(ctx, job)
			require.NoError(t, err)
		}

		// Get stats for the queue
		stat, err := client.Stat(ctx, storj.PlacementConstraint(42), false)
		require.NoError(t, err)
		require.Equal(t, jobq.QueueStat{
			Placement:        42,
			Count:            5,
			MinInsertedAt:    stat.MinInsertedAt,
			MaxInsertedAt:    stat.MaxInsertedAt,
			MinAttemptedAt:   nil,
			MaxAttemptedAt:   nil,
			MinSegmentHealth: 0.0,
			MaxSegmentHealth: 4.0,
		}, stat)
		require.GreaterOrEqual(t, stat.MaxInsertedAt, stat.MinInsertedAt)
	})
}

func TestStatAll(t *testing.T) {
	jobqtest.WithServerAndClient(t, nil, func(ctx *testcontext.Context, server *jobqtest.TestServer, client *jobq.Client) {
		// Create and push a few jobs
		for i := 0; i < 5; i++ {
			job := jobq.RepairJob{
				ID:                       jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: uint64(i)},
				Health:                   float64(i),
				Placement:                42,
				InsertedAt:               uint64(time.Now().Unix()),
				NumNormalizedHealthy:     int16(i),
				NumNormalizedRetrievable: int16(i * 2),
				NumOutOfPlacement:        2,
			}
			_, err := client.Push(ctx, job)
			require.NoError(t, err)
		}
		job := jobq.RepairJob{
			ID:         jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: 5},
			Health:     10.0,
			Placement:  43,
			InsertedAt: uint64(time.Now().Unix()),
		}
		_, err := client.Push(ctx, job)
		require.NoError(t, err)

		// Get stats for the queue without histogram
		stat, err := client.StatAll(ctx, false)
		require.NoError(t, err)
		require.Len(t, stat, 4)

		sort.Slice(stat, func(i, j int) bool {
			if stat[i].Placement < stat[j].Placement {
				return true
			}
			if stat[i].Placement > stat[j].Placement {
				return false
			}
			if stat[i].MinAttemptedAt == nil || stat[j].MinAttemptedAt != nil {
				return true
			}
			if stat[i].MinAttemptedAt != nil || stat[j].MinAttemptedAt == nil {
				return false
			}
			return stat[i].MinSegmentHealth < stat[j].MinSegmentHealth
		})

		// no jobs in retry for placement 42
		require.Equal(t, jobq.QueueStat{
			Placement:        42,
			Count:            0,
			MinInsertedAt:    time.Unix(0, 0).UTC(),
			MaxInsertedAt:    time.Unix(0, 0).UTC(),
			MinAttemptedAt:   nil,
			MaxAttemptedAt:   nil,
			MinSegmentHealth: 0.0,
			MaxSegmentHealth: 0.0,
		}, stat[0])

		require.Equal(t, jobq.QueueStat{
			Placement:        42,
			Count:            5,
			MinInsertedAt:    stat[1].MinInsertedAt.UTC(),
			MaxInsertedAt:    stat[1].MaxInsertedAt.UTC(),
			MinAttemptedAt:   nil,
			MaxAttemptedAt:   nil,
			MinSegmentHealth: 0.0,
			MaxSegmentHealth: 4.0,
		}, stat[1])
		require.NotZero(t, stat[1].MinInsertedAt)
		require.NotZero(t, stat[1].MaxInsertedAt)
		require.LessOrEqual(t, stat[1].MinInsertedAt, stat[1].MaxInsertedAt)

		// no jobs in retry for placement 43
		require.Equal(t, jobq.QueueStat{
			Placement:        43,
			Count:            0,
			MinInsertedAt:    time.Unix(0, 0).UTC(),
			MaxInsertedAt:    time.Unix(0, 0).UTC(),
			MinAttemptedAt:   nil,
			MaxAttemptedAt:   nil,
			MinSegmentHealth: 0.0,
			MaxSegmentHealth: 0.0,
		}, stat[2])

		require.Equal(t, jobq.QueueStat{
			Placement:        43,
			Count:            1,
			MinInsertedAt:    stat[3].MinInsertedAt.UTC(),
			MaxInsertedAt:    stat[3].MaxInsertedAt.UTC(),
			MinAttemptedAt:   nil,
			MaxAttemptedAt:   nil,
			MinSegmentHealth: 10.0,
			MaxSegmentHealth: 10.0,
		}, stat[3])
		require.NotZero(t, stat[3].MinInsertedAt)
		require.NotZero(t, stat[3].MaxInsertedAt)
		require.LessOrEqual(t, stat[3].MinInsertedAt, stat[3].MaxInsertedAt)

		// Now test with histogram enabled
		statWithHistogram, err := client.StatAll(ctx, true)
		require.NoError(t, err)
		require.Len(t, statWithHistogram, 4)

		// Sort the stats the same way as before
		sort.Slice(statWithHistogram, func(i, j int) bool {
			if statWithHistogram[i].Placement < statWithHistogram[j].Placement {
				return true
			}
			if statWithHistogram[i].Placement > statWithHistogram[j].Placement {
				return false
			}
			if statWithHistogram[i].MinAttemptedAt == nil || statWithHistogram[j].MinAttemptedAt != nil {
				return true
			}
			if statWithHistogram[i].MinAttemptedAt != nil || statWithHistogram[j].MinAttemptedAt == nil {
				return false
			}
			return statWithHistogram[i].MinSegmentHealth < statWithHistogram[j].MinSegmentHealth
		})

		// Check the non-empty queues for histogram data
		for _, s := range statWithHistogram {
			if s.Count > 0 {
				// Queues with jobs should have histogram data
				require.NotEmpty(t, s.Histogram, "Queue with placement %d should have histogram data", s.Placement)

				// Check histogram fields
				for _, histItem := range s.Histogram {
					require.NotZero(t, histItem.Count, "Histogram item count should be non-zero")
					require.NotZero(t, histItem.Exemplar.StreamID, "Histogram item examplar StreamID should be non-zero")

					// These might be zero in some cases, but should be non-negative
					require.GreaterOrEqual(t, histItem.NumNormalizedHealthy, int64(0), "NumNormalizedHealthy should be non-negative")
					require.GreaterOrEqual(t, histItem.NumOutOfPlacement, int64(0), "NumOutOfPlacement should be non-negative")
					require.GreaterOrEqual(t, histItem.NumNormalizedRetrievable, int64(0), "NumNormalizedRetrievable should be non-negative")
				}
			} else {
				// Empty queues should have empty histograms
				require.Empty(t, s.Histogram, "Empty queue with placement %d should have empty histogram", s.Placement)
			}
		}
	})
}
