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

		err = cli.AddPlacementQueue(ctx, 42)
		require.NoError(t, err)

		job := jobq.RepairJob{
			ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: 2},
			Priority:  3.0,
			Placement: 42,
		}
		wasNew, err := cli.Push(ctx, job)
		require.NoError(t, err)
		require.True(t, wasNew)

		gotRepairLen, gotRetryLen, err := cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(1), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)

		gotJob, err := cli.Inspect(ctx, 42, job.ID.StreamID, 2)
		require.NoError(t, err)
		require.NotZero(t, gotJob.InsertedAt)
		job.InsertedAt = gotJob.InsertedAt
		job.UpdatedAt = gotJob.UpdatedAt
		require.Equal(t, job, gotJob)

		gotJob, err = cli.Pop(ctx, nil, nil)
		require.NoError(t, err)
		require.Equal(t, job.ID.StreamID, gotJob.ID.StreamID)
		require.Equal(t, uint64(2), gotJob.ID.Position)
		require.Equal(t, 3.0, gotJob.Priority)
		require.Equal(t, uint16(42), gotJob.Placement)

		gotRepairLen, gotRetryLen, err = cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(0), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)

		gotJob, err = cli.Pop(ctx, nil, nil)
		require.Error(t, err)
		require.Empty(t, gotJob.ID)
		require.ErrorIs(t, err, jobq.ErrQueueEmpty)
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

		// Add a placement queue
		err = cli.AddPlacementQueue(ctx, 42)
		require.NoError(t, err)

		// Create multiple jobs
		jobs := []jobq.RepairJob{
			{
				ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: 2},
				Priority:  3.0,
				Placement: 42,
			},
			{
				ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: 3},
				Priority:  4.0,
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
		gotJob1, err := cli.Pop(ctx, nil, nil)
		require.NoError(t, err)
		gotJob2, err := cli.Pop(ctx, nil, nil)
		require.NoError(t, err)

		// Higher priority job should come first
		require.Equal(t, jobs[1].ID.StreamID, gotJob1.ID.StreamID)
		require.Equal(t, jobs[0].ID.StreamID, gotJob2.ID.StreamID)
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

		// Add a placement queue
		err = cli.AddPlacementQueue(ctx, 42)
		require.NoError(t, err)

		// Create and push a job
		job := jobq.RepairJob{
			ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: 2},
			Priority:  3.0,
			Placement: 42,
		}
		wasNew, err := cli.Push(ctx, job)
		require.NoError(t, err)
		require.True(t, wasNew)

		// Peek the job
		peekedJob, err := cli.Peek(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, job.ID.StreamID, peekedJob.ID.StreamID)
		require.Equal(t, job.ID.Position, peekedJob.ID.Position)
		require.Equal(t, job.Priority, peekedJob.Priority)

		// Verify the job is still in the queue after peeking
		gotRepairLen, gotRetryLen, err := cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(1), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)

		// Pop the job to ensure it's still there
		gotJob, err := cli.Pop(ctx, nil, nil)
		require.NoError(t, err)
		require.Equal(t, job.ID.StreamID, gotJob.ID.StreamID)
		require.Equal(t, job.ID.Position, gotJob.ID.Position)
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

		// Add a placement queue
		err = cli.AddPlacementQueue(ctx, 42)
		require.NoError(t, err)

		// Create and push a few jobs
		for i := 0; i < 5; i++ {
			job := jobq.RepairJob{
				ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: uint64(i)},
				Priority:  float64(i),
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
		_, err = cli.Pop(ctx, nil, nil)
		require.ErrorIs(t, err, jobq.ErrQueueEmpty)
	}()

	cancel()
	require.NoError(t, group.Wait())
}

func TestClientServerDestroyPlacementQueue(t *testing.T) {
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

		// Add a placement queue
		err = cli.AddPlacementQueue(ctx, 42)
		require.NoError(t, err)

		// Create and push a job
		job := jobq.RepairJob{
			ID:        jobq.SegmentIdentifier{StreamID: testrand.UUID(), Position: 2},
			Priority:  3.0,
			Placement: 42,
		}
		wasNew, err := cli.Push(ctx, job)
		require.NoError(t, err)
		require.True(t, wasNew)

		// Destroy the placement queue
		err = cli.DestroyPlacementQueue(ctx, 42)
		require.NoError(t, err)

		// Try to get length - should fail because queue doesn't exist
		_, _, err = cli.Len(ctx, 42)
		require.Error(t, err)

		// Add the placement queue again
		err = cli.AddPlacementQueue(ctx, 42)
		require.NoError(t, err)

		// Verify it's empty
		gotRepairLen, gotRetryLen, err := cli.Len(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int64(0), gotRepairLen)
		require.Equal(t, int64(0), gotRetryLen)
	}()

	cancel()
	require.NoError(t, group.Wait())
}
