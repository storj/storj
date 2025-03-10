// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobq

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/drpc/drpcconn"
	pb "storj.io/storj/satellite/internalpb"
)

// ErrQueueEmpty is returned by the client when the queue is empty.
var ErrQueueEmpty = errors.New("queue is empty")

// ErrJobNotFound is returned by the client when a particular job is not found.
var ErrJobNotFound = errors.New("job not found")

// Client wraps a DRPCJobQueueClient.
type Client struct {
	client pb.DRPCJobQueueClient
	conn   *drpcconn.Conn
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	conn := c.conn
	c.conn = nil
	return conn.Close()
}

// Push adds a new item to the job queue with the given health.
//
// It returns an indication of whether the given segment was newly inserted or
// if it already existed in the target queue.
func (c *Client) Push(ctx context.Context, job RepairJob) (wasNew bool, err error) {
	resp, err := c.client.Push(ctx, &pb.JobQueuePushRequest{
		Job: ConvertJobToProtobuf(job),
	})
	if err != nil {
		return false, fmt.Errorf("could not push repair job: %w", err)
	}
	return resp.NewlyInserted, nil
}

// PushBatch adds multiple items to the appropriate job queues with the given
// health values.
//
// It returns a slice of booleans indicating whether each segment was newly
// inserted or if it already existed in the target queue.
func (c *Client) PushBatch(ctx context.Context, jobs []RepairJob) (wasNew []bool, err error) {
	req := &pb.JobQueuePushBatchRequest{
		Jobs: make([]*pb.RepairJob, len(jobs)),
	}
	for i, job := range jobs {
		req.Jobs[i] = ConvertJobToProtobuf(job)
	}
	resp, err := c.client.PushBatch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("could not push repair jobs: %w", err)
	}
	return resp.NewlyInserted, nil
}

// Pop removes and returns the lowest-health item from the job queue. If there
// are no items in the queue, it returns ErrQueueEmpty.
func (c *Client) Pop(ctx context.Context, includedPlacements, excludedPlacements []storj.PlacementConstraint) (job RepairJob, err error) {
	resp, err := c.client.Pop(ctx, &pb.JobQueuePopRequest{
		IncludedPlacements: placementConstraintsToInt32Slice(includedPlacements),
		ExcludedPlacements: placementConstraintsToInt32Slice(excludedPlacements),
	})
	if err != nil {
		return RepairJob{}, fmt.Errorf("could not pop repair job: %w", err)
	}
	job, err = ConvertJobFromProtobuf(resp.Job)
	if err != nil {
		return RepairJob{}, fmt.Errorf("invalid repair job: %w", err)
	}
	if job.ID.StreamID.IsZero() {
		return job, ErrQueueEmpty
	}
	return job, nil
}

// Peek returns the lowest-health item from the job queue without removing
// it. If there are no items in the queue, it returns ErrQueueEmpty.
func (c *Client) Peek(ctx context.Context, placement storj.PlacementConstraint) (job RepairJob, err error) {
	resp, err := c.client.Peek(ctx, &pb.JobQueuePeekRequest{
		Placement: int32(placement),
	})
	if err != nil {
		return RepairJob{}, fmt.Errorf("could not peek repair job: %w", err)
	}
	job, err = ConvertJobFromProtobuf(resp.Job)
	if err != nil {
		return RepairJob{}, fmt.Errorf("invalid repair job: %w", err)
	}
	if job.ID.StreamID.IsZero() {
		return job, ErrQueueEmpty
	}
	return job, nil
}

// Inspect finds a job in the queue by streamID and position and returns all of
// the job information. If the job is not found, it returns ErrJobNotFound.
func (c *Client) Inspect(ctx context.Context, placement storj.PlacementConstraint, streamID uuid.UUID, position uint64) (job RepairJob, err error) {
	resp, err := c.client.Inspect(ctx, &pb.JobQueueInspectRequest{
		Placement: int32(placement),
		StreamId:  streamID[:],
		Position:  position,
	})
	if err != nil {
		return RepairJob{}, fmt.Errorf("could not inspect repair job: %w", err)
	}
	job, err = ConvertJobFromProtobuf(resp.Job)
	if err != nil {
		return RepairJob{}, fmt.Errorf("invalid repair job: %w", err)
	}
	if job.ID.StreamID.IsZero() {
		return job, ErrJobNotFound
	}
	return job, nil
}

// Len returns the number of items in the indicated job queue.
func (c *Client) Len(ctx context.Context, placement storj.PlacementConstraint) (repairLen, retryLen int64, err error) {
	resp, err := c.client.Len(ctx, &pb.JobQueueLengthRequest{
		Placement: int32(placement),
	})
	if err != nil {
		return 0, 0, err
	}
	return resp.RepairLength, resp.RetryLength, nil
}

// LenAll sums up the number of items in all queues on the server.
func (c *Client) LenAll(ctx context.Context) (repairLen, retryLen int64, err error) {
	resp, err := c.client.Len(ctx, &pb.JobQueueLengthRequest{
		AllPlacements: true,
	})
	if err != nil {
		return 0, 0, err
	}
	return resp.RepairLength, resp.RetryLength, nil
}

// AddPlacementQueue adds a new queue for the given placement.
func (c *Client) AddPlacementQueue(ctx context.Context, placement storj.PlacementConstraint) error {
	_, err := c.client.AddPlacementQueue(ctx, &pb.JobQueueAddPlacementQueueRequest{
		Placement: int32(placement),
	})
	return err
}

// DestroyPlacementQueue truncates and removes the queue for the given placement.
func (c *Client) DestroyPlacementQueue(ctx context.Context, placement storj.PlacementConstraint) error {
	_, err := c.client.DestroyPlacementQueue(ctx, &pb.JobQueueDestroyPlacementQueueRequest{
		Placement: int32(placement),
	})
	return err
}

// Truncate removes all items from a job queue.
func (c *Client) Truncate(ctx context.Context, placement storj.PlacementConstraint) error {
	_, err := c.client.Truncate(ctx, &pb.JobQueueTruncateRequest{
		Placement: int32(placement),
	})
	return err
}

// TruncateAll removes all items from all job queues on a server.
func (c *Client) TruncateAll(ctx context.Context) error {
	_, err := c.client.Truncate(ctx, &pb.JobQueueTruncateRequest{
		AllPlacements: true,
	})
	return err
}

// Clean removes all jobs with UpdatedAt time before the given cutoff.
func (c *Client) Clean(ctx context.Context, placement storj.PlacementConstraint, updatedBefore time.Time) (removedSegments int32, err error) {
	resp, err := c.client.Clean(ctx, &pb.JobQueueCleanRequest{
		Placement:     int32(placement),
		UpdatedBefore: updatedBefore,
	})
	if err != nil {
		return 0, fmt.Errorf("could not clean jobs: %w", err)
	}
	return resp.RemovedSegments, nil
}

// CleanAll removes all jobs with UpdatedAt time before the given cutoff from
// all placement queues.
func (c *Client) CleanAll(ctx context.Context, updatedBefore time.Time) (removedSegments int32, err error) {
	resp, err := c.client.Clean(ctx, &pb.JobQueueCleanRequest{
		UpdatedBefore: updatedBefore,
		AllPlacements: true,
	})
	if err != nil {
		return 0, fmt.Errorf("could not clean all jobs: %w", err)
	}
	return resp.RemovedSegments, nil
}

// Trim removes all jobs with Health greater than the given threshold.
func (c *Client) Trim(ctx context.Context, placement storj.PlacementConstraint, healthGreaterThan float64) (removedSegments int32, err error) {
	resp, err := c.client.Trim(ctx, &pb.JobQueueTrimRequest{
		Placement:         int32(placement),
		HealthGreaterThan: healthGreaterThan,
	})
	if err != nil {
		return 0, err
	}
	return resp.RemovedSegments, nil
}

// TrimAll removes all jobs with UpdatedAt time before the given cutoff from
// all placement queues.
func (c *Client) TrimAll(ctx context.Context, healthGreaterThan float64) (removedSegments int32, err error) {
	resp, err := c.client.Trim(ctx, &pb.JobQueueTrimRequest{
		HealthGreaterThan: healthGreaterThan,
		AllPlacements:     true,
	})
	if err != nil {
		return 0, err
	}
	return resp.RemovedSegments, nil
}

// Dial dials an address and creates a new client.
func Dial(addr net.Addr) (*Client, error) {
	rawConn, err := net.Dial(addr.Network(), addr.String())
	if err != nil {
		return nil, fmt.Errorf("dialing %q: %w", addr, err)
	}
	conn := drpcconn.New(rawConn)
	return &Client{
		client: pb.NewDRPCJobQueueClient(conn),
		conn:   conn,
	}, nil
}

// WrapConn wraps an existing connection in a client.
func WrapConn(rawConn net.Conn) *Client {
	conn := drpcconn.New(rawConn)
	return &Client{
		client: pb.NewDRPCJobQueueClient(conn),
		conn:   conn,
	}
}

func placementConstraintsToInt32Slice(placements []storj.PlacementConstraint) []int32 {
	slice := make([]int32, len(placements))
	for i, c := range placements {
		slice[i] = int32(c)
	}
	return slice
}
