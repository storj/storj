// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobq

import (
	"context"
	"errors"
	"fmt"
	"time"

	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	pb "storj.io/storj/satellite/internalpb"
)

// ErrQueueEmpty is returned by the client when the queue is empty.
var ErrQueueEmpty = errors.New("queue is empty")

// ErrJobNotFound is returned by the client when a particular job is not found.
var ErrJobNotFound = errors.New("job not found")

// Client wraps a DRPCJobQueueClient.
type Client struct {
	client pb.DRPCJobQueueClient
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	conn := c.client.DRPCConn()
	c.client = nil
	if conn != nil {
		return conn.Close()
	}
	return nil
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

// Pop removes and returns the 'limit' lowest-health items from the indicated
// job queues. If there are less than 'limit' items in the queue, it removes
// and returns all of them.
func (c *Client) Pop(ctx context.Context, limit int, includedPlacements, excludedPlacements []storj.PlacementConstraint) (jobs []RepairJob, err error) {
	resp, err := c.client.Pop(ctx, &pb.JobQueuePopRequest{
		IncludedPlacements: placementConstraintsToInt32Slice(includedPlacements),
		ExcludedPlacements: placementConstraintsToInt32Slice(excludedPlacements),
		Limit:              int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("could not pop repair jobs: %w", err)
	}
	jobs = make([]RepairJob, 0, len(resp.Jobs))
	var errList []error
	for _, j := range resp.Jobs {
		job, err := ConvertJobFromProtobuf(j)
		if err != nil {
			errList = append(errList, fmt.Errorf("invalid repair job: %w", err))
			continue
		}
		jobs = append(jobs, job)
	}
	return jobs, errors.Join(errList...)
}

// Peek returns the 'limit' lowest-health items from the indicated job queues
// without removing them. If there are less than 'limit' items in all the
// queues, it returns all of them.
func (c *Client) Peek(ctx context.Context, limit int, includedPlacements, excludedPlacements []storj.PlacementConstraint) (jobs []RepairJob, err error) {
	resp, err := c.client.Peek(ctx, &pb.JobQueuePeekRequest{
		IncludedPlacements: placementConstraintsToInt32Slice(includedPlacements),
		ExcludedPlacements: placementConstraintsToInt32Slice(excludedPlacements),
		Limit:              int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("could not peek repair jobs: %w", err)
	}
	jobs = make([]RepairJob, 0, len(resp.Jobs))
	var errList []error
	for _, j := range resp.Jobs {
		job, err := ConvertJobFromProtobuf(j)
		if err != nil {
			errList = append(errList, fmt.Errorf("invalid repair job: %w", err))
			continue
		}
		jobs = append(jobs, job)
	}
	return jobs, errors.Join(errList...)
}

// Inspect finds a job in the queue by streamID, position, and placement, and
// returns all of the job information. If the job is not found, it returns
// ErrJobNotFound.
func (c *Client) Inspect(ctx context.Context, placement storj.PlacementConstraint, streamID uuid.UUID, position uint64) (job RepairJob, err error) {
	resp, err := c.client.Inspect(ctx, &pb.JobQueueInspectRequest{
		Placement: int32(placement),
		StreamId:  streamID[:],
		Position:  position,
	})
	if err != nil {
		return RepairJob{}, fmt.Errorf("could not inspect repair job: %w", err)
	}
	if !resp.Found {
		return job, ErrJobNotFound
	}
	job, err = ConvertJobFromProtobuf(resp.Job)
	if err != nil {
		return RepairJob{}, fmt.Errorf("invalid repair job: %w", err)
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

// Delete removes a specific job from the indicated queue.
func (c *Client) Delete(ctx context.Context, placement storj.PlacementConstraint, streamID uuid.UUID, position uint64) (wasDeleted bool, err error) {
	resp, err := c.client.Delete(ctx, &pb.JobQueueDeleteRequest{
		Placement: int32(placement),
		StreamId:  streamID[:],
		Position:  position,
	})
	if err != nil {
		return false, err
	}
	return resp.DidDelete, nil
}

// Stat collects statistics about the indicated job queue.
func (c *Client) Stat(ctx context.Context, placement storj.PlacementConstraint, withHistogram bool) (stat QueueStat, err error) {
	resp, err := c.client.Stat(ctx, &pb.JobQueueStatRequest{
		Placement:     int32(placement),
		WithHistogram: withHistogram,
	})
	if err != nil {
		return stat, err
	}
	s := resp.Stats[0]
	return QueueStat{
		Placement:        storj.PlacementConstraint(s.Placement),
		Count:            s.Count,
		MaxInsertedAt:    s.MaxInsertedAt,
		MinInsertedAt:    s.MinInsertedAt,
		MaxAttemptedAt:   s.MaxAttemptedAt,
		MinAttemptedAt:   s.MinAttemptedAt,
		MinSegmentHealth: s.MinSegmentHealth,
		MaxSegmentHealth: s.MaxSegmentHealth,
		Histogram:        histogramItemFromProtobuf(s.Histogram),
	}, nil
}

// StatAll collects statistics about all job queues on a server.
func (c *Client) StatAll(ctx context.Context, withHistogram bool) (stats []QueueStat, err error) {
	resp, err := c.client.Stat(ctx, &pb.JobQueueStatRequest{
		AllPlacements: true,
		WithHistogram: withHistogram,
	})
	if err != nil {
		return nil, err
	}
	stats = make([]QueueStat, 0, len(resp.Stats))
	for _, s := range resp.Stats {
		stats = append(stats, QueueStat{
			Placement:        storj.PlacementConstraint(s.Placement),
			Count:            s.Count,
			MaxInsertedAt:    s.MaxInsertedAt,
			MinInsertedAt:    s.MinInsertedAt,
			MaxAttemptedAt:   s.MaxAttemptedAt,
			MinAttemptedAt:   s.MinAttemptedAt,
			MinSegmentHealth: s.MinSegmentHealth,
			MaxSegmentHealth: s.MaxSegmentHealth,
			Histogram:        histogramItemFromProtobuf(s.Histogram),
		})
	}
	return stats, nil
}

func histogramItemFromProtobuf(protoItem []*pb.QueueStatHistogram) (res []HistogramItem) {
	for _, h := range protoItem {
		streamID, err := uuid.FromBytes(h.ExemplarStreamId)
		if err != nil {
			return res
		}
		res = append(res, HistogramItem{
			Count:                    h.Count,
			NumOutOfPlacement:        int64(h.NumOutOfPlacement),
			NumNormalizedRetrievable: int64(h.NumNormalizedRetrievable),
			NumNormalizedHealthy:     int64(h.NumNormalizedHealthy),
			Exemplar: SegmentIdentifier{
				StreamID: streamID,
				Position: h.ExemplarPosition,
			},
		})
	}
	return res
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

// TestingSetAttemptedTime sets the LastAttemptedAt field of a specific job.
// This is only intended for testing scenarios.
func (c *Client) TestingSetAttemptedTime(ctx context.Context, placement storj.PlacementConstraint, streamID uuid.UUID, position uint64, t time.Time) (rowsAffected int64, err error) {
	resp, err := c.client.TestingSetAttemptedTime(ctx, &pb.JobQueueTestingSetAttemptedTimeRequest{
		Placement: int32(placement),
		StreamId:  streamID[:],
		Position:  position,
		NewTime:   t,
	})
	if err != nil {
		return 0, err
	}
	return int64(resp.RowsAffected), nil
}

// TestingSetUpdatedTime sets the UpdatedAt field of a specific job.
// This is only intended for testing scenarios.
func (c *Client) TestingSetUpdatedTime(ctx context.Context, placement storj.PlacementConstraint, streamID uuid.UUID, position uint64, t time.Time) (rowsAffected int64, err error) {
	resp, err := c.client.TestingSetUpdatedTime(ctx, &pb.JobQueueTestingSetUpdatedTimeRequest{
		Placement: int32(placement),
		StreamId:  streamID[:],
		Position:  position,
		NewTime:   t,
	})
	if err != nil {
		return 0, fmt.Errorf("could not set updated time for job: %w", err)
	}
	return int64(resp.RowsAffected), nil
}

// WrapConn wraps an existing connection in a client.
func WrapConn(conn *rpc.Conn) *Client {
	return &Client{
		client: pb.NewDRPCJobQueueClient(conn),
	}
}

// NewDialer creates a new dialer for the job queue client.
func NewDialer(tlsOpts *tlsopts.Options) rpc.Dialer {
	dialer := rpc.NewDefaultDialer(tlsOpts)
	dialer.Pool = rpc.NewDefaultConnectionPool()
	dialer.DialTimeout = time.Hour
	connector := rpc.NewHybridConnector()
	connector.SetSendDRPCMuxHeader(true)
	dialer.Connector = connector
	return dialer
}

func placementConstraintsToInt32Slice(placements []storj.PlacementConstraint) []int32 {
	slice := make([]int32, len(placements))
	for i, c := range placements {
		slice[i] = int32(c)
	}
	return slice
}
