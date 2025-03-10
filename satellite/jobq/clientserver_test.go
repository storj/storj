// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobq_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/server"
)

func TestClientAndServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := zaptest.NewLogger(t)
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv, err := server.New(log, addr, nil, time.Hour, 1e8, 1e6)
	require.NoError(t, err)

	var group errgroup.Group
	group.Go(func() error {
		return srv.Run(ctx)
	})

	func() {
		cli, err := jobq.Dial(srv.Addr())
		require.NoError(t, err)
		defer func() { require.NoError(t, cli.Close()) }()

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
	}()

	cancel()
	require.NoError(t, group.Wait())
}

func TestClientServerPushBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := zaptest.NewLogger(t)
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv, err := server.New(log, addr, nil, time.Hour, 1e8, 1e6)
	require.NoError(t, err)

	var group errgroup.Group
	group.Go(func() error {
		return srv.Run(ctx)
	})

	func() {
		cli, err := jobq.Dial(srv.Addr())
		require.NoError(t, err)
		defer func() { require.NoError(t, cli.Close()) }()

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
	}()

	cancel()
	require.NoError(t, group.Wait())
}

func TestClientServerPeek(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := zaptest.NewLogger(t)
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv, err := server.New(log, addr, nil, time.Hour, 1e8, 1e6)
	require.NoError(t, err)

	var group errgroup.Group
	group.Go(func() error {
		return srv.Run(ctx)
	})

	func() {
		cli, err := jobq.Dial(srv.Addr())
		require.NoError(t, err)
		defer func() { require.NoError(t, cli.Close()) }()

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
	}()

	cancel()
	require.NoError(t, group.Wait())
}

func TestClientServerTruncate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := zaptest.NewLogger(t)
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv, err := server.New(log, addr, nil, time.Hour, 1e8, 1e6)
	require.NoError(t, err)

	var group errgroup.Group
	group.Go(func() error {
		return srv.Run(ctx)
	})

	func() {
		cli, err := jobq.Dial(srv.Addr())
		require.NoError(t, err)
		defer func() { require.NoError(t, cli.Close()) }()

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
	}()

	cancel()
	require.NoError(t, group.Wait())
}

func TestClientServerClean(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := zaptest.NewLogger(t)
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv, err := server.New(log, addr, nil, time.Hour, 1e8, 1e6)
	require.NoError(t, err)

	var group errgroup.Group
	group.Go(func() error {
		return srv.Run(ctx)
	})

	func() {
		cli, err := jobq.Dial(srv.Addr())
		require.NoError(t, err)
		defer func() { require.NoError(t, cli.Close()) }()

		// Set up our time control
		now := time.Now()
		timeIncrement := time.Duration(0)
		timeFunc := func() time.Time {
			return now.Add(timeIncrement)
		}
		// make the queue for placement 42 get initialized now, so we can change
		// its time function
		_, err = srv.QueueMap.GetQueue(42)
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
	}()

	cancel()
	require.NoError(t, group.Wait())
}

func TestClientServerTrim(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := zaptest.NewLogger(t)
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv, err := server.New(log, addr, nil, time.Hour, 1e8, 1e6)
	require.NoError(t, err)

	var group errgroup.Group
	group.Go(func() error {
		return srv.Run(ctx)
	})

	func() {
		cli, err := jobq.Dial(srv.Addr())
		require.NoError(t, err)
		defer func() { require.NoError(t, cli.Close()) }()

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
			_, err = cli.Push(ctx, job)
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
	}()

	cancel()
	require.NoError(t, group.Wait())
}
