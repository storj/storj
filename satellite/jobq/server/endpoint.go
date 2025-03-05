// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	pb "storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/jobqueue"
)

// JobqEndpoint implements the DRPCJobQueueServer interface.
type JobqEndpoint struct {
	log    *zap.Logger
	queues *QueueMap
}

// Push inserts a job into the appropriate queue for its placement.
func (se *JobqEndpoint) Push(ctx context.Context, req *pb.JobQueuePushRequest) (*pb.JobQueuePushResponse, error) {
	reqJob := req.GetJob()
	if reqJob == nil {
		return nil, fmt.Errorf("missing job")
	}
	q := se.queues.GetQueue(storj.PlacementConstraint(reqJob.Placement))
	if q == nil {
		return nil, fmt.Errorf("no queue for placement %v", reqJob.Placement)
	}

	job, err := jobq.ConvertJobFromProtobuf(reqJob)
	if err != nil {
		return nil, fmt.Errorf("invalid job: %w", err)
	}
	wasNew := q.Insert(job)
	return &pb.JobQueuePushResponse{
		NewlyInserted: wasNew,
	}, nil
}

// PushBatch inserts multiple jobs into the appropriate queues for their
// placements.
func (se *JobqEndpoint) PushBatch(ctx context.Context, req *pb.JobQueuePushBatchRequest) (*pb.JobQueuePushBatchResponse, error) {
	encounteredErrors := []error{}
	nonNilErrors := false
	wasNewList := make([]bool, len(req.Jobs))
	for i, reqJob := range req.Jobs {
		q := se.queues.GetQueue(storj.PlacementConstraint(reqJob.Placement))
		if q == nil {
			encounteredErrors = append(encounteredErrors, fmt.Errorf("no queue for placement %v", reqJob.Placement))
			nonNilErrors = true
			continue
		}
		job, err := jobq.ConvertJobFromProtobuf(reqJob)
		if err != nil {
			encounteredErrors = append(encounteredErrors, fmt.Errorf("invalid job: %w", err))
			nonNilErrors = true
			continue
		}
		wasNewList[i] = q.Insert(job)
		encounteredErrors = append(encounteredErrors, nil)
	}
	if nonNilErrors {
		return nil, fmt.Errorf("could not push all jobs to queues: %v", encounteredErrors)
	}
	return &pb.JobQueuePushBatchResponse{
		NewlyInserted: wasNewList,
	}, nil
}

// Pop removes the lowest-health job from the queues for the requested
// placements.
func (se *JobqEndpoint) Pop(ctx context.Context, req *pb.JobQueuePopRequest) (*pb.JobQueuePopResponse, error) {
	if len(req.IncludedPlacements) == 1 {
		// we can optimize for this common case by going directly to that queue
		q := se.queues.GetQueue(storj.PlacementConstraint(req.IncludedPlacements[0]))
		if q == nil {
			return nil, fmt.Errorf("no queue for placement %v", req.IncludedPlacements[0])
		}
		job := q.Pop()
		return &pb.JobQueuePopResponse{Job: jobq.ConvertJobToProtobuf(job)}, nil
	}

	// otherwise we need to check all requested queues for the lowest health match
	queues := se.queues.GetAllQueues()
	if len(req.IncludedPlacements) > 0 {
		newQueues := make(map[storj.PlacementConstraint]*jobqueue.Queue)
		for _, placement := range req.IncludedPlacements {
			if q, ok := queues[storj.PlacementConstraint(placement)]; ok {
				newQueues[storj.PlacementConstraint(placement)] = q
			}
		}
		queues = newQueues
	} else {
		for _, placement := range req.ExcludedPlacements {
			delete(queues, storj.PlacementConstraint(placement))
		}
	}

	var bestResult jobq.RepairJob
	for _, q := range queues {
		job := q.Pop()
		if !job.ID.StreamID.IsZero() && (bestResult.ID.StreamID.IsZero() || job.Health < bestResult.Health) {
			bestResult = job
		}
	}
	return &pb.JobQueuePopResponse{Job: jobq.ConvertJobToProtobuf(bestResult)}, nil
}

// Peek returns the lowest-health job from the queues for the requested
// placements without removing the job from its queue.
func (se *JobqEndpoint) Peek(ctx context.Context, req *pb.JobQueuePeekRequest) (*pb.JobQueuePeekResponse, error) {
	q := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if q == nil {
		return nil, fmt.Errorf("no queue for placement %v", req.Placement)
	}
	job := q.Peek()
	return &pb.JobQueuePeekResponse{Job: jobq.ConvertJobToProtobuf(job)}, nil
}

// Len returns the number of jobs in the queues for the requested placement.
func (se *JobqEndpoint) Len(ctx context.Context, req *pb.JobQueueLengthRequest) (*pb.JobQueueLengthResponse, error) {
	q := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if q == nil {
		return nil, fmt.Errorf("no queue for placement %v", req.Placement)
	}
	repairLen, retryLen := q.Len()
	return &pb.JobQueueLengthResponse{RepairLength: repairLen, RetryLength: retryLen}, nil
}

// Inspect finds a particular job in the queue by its placement, streamID, and
// position and returns all of the job information.
func (se *JobqEndpoint) Inspect(ctx context.Context, req *pb.JobQueueInspectRequest) (*pb.JobQueueInspectResponse, error) {
	streamID, err := uuid.FromBytes(req.StreamId)
	if err != nil {
		return nil, fmt.Errorf("invalid stream id %x: %w", req.StreamId, err)
	}
	position := req.Position
	q := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if q == nil {
		return nil, fmt.Errorf("no queue for placement %v", req.Placement)
	}
	job := q.Inspect(streamID, position)
	return &pb.JobQueueInspectResponse{Job: jobq.ConvertJobToProtobuf(job)}, nil
}

// Truncate removes all jobs from the queue for the requested placement. The
// queue is not destroyed.
func (se *JobqEndpoint) Truncate(ctx context.Context, req *pb.JobQueueTruncateRequest) (*pb.JobQueueTruncateResponse, error) {
	q := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if q == nil {
		return nil, fmt.Errorf("no queue for placement %v", req.Placement)
	}
	q.Truncate()
	return &pb.JobQueueTruncateResponse{}, nil
}

// AddPlacementQueue creates a new queue for the requested placement.
func (se *JobqEndpoint) AddPlacementQueue(ctx context.Context, req *pb.JobQueueAddPlacementQueueRequest) (*pb.JobQueueAddPlacementQueueResponse, error) {
	placement := storj.PlacementConstraint(req.Placement)
	err := se.queues.AddQueue(placement)
	if err != nil {
		return nil, fmt.Errorf("failed to add queue: %w", err)
	}
	return &pb.JobQueueAddPlacementQueueResponse{}, nil
}

// DestroyPlacementQueue removes the queue for the requested placement.
func (se *JobqEndpoint) DestroyPlacementQueue(ctx context.Context, req *pb.JobQueueDestroyPlacementQueueRequest) (*pb.JobQueueDestroyPlacementQueueResponse, error) {
	placement := storj.PlacementConstraint(req.Placement)
	err := se.queues.DestroyQueue(placement)
	if err != nil {
		return nil, fmt.Errorf("failed to destroy queue: %w", err)
	}
	return &pb.JobQueueDestroyPlacementQueueResponse{}, nil
}

// Clean removes all jobs from the queue that were last updated before the
// requested time. If the given placement is negative, all queues are cleaned.
func (se *JobqEndpoint) Clean(ctx context.Context, req *pb.JobQueueCleanRequest) (*pb.JobQueueCleanResponse, error) {
	// req.Placement < 0 is deprecated; use AllPlacements
	if req.Placement < 0 || req.AllPlacements {
		return se.cleanAll(req.UpdatedBefore)
	}
	q := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if q == nil {
		return nil, fmt.Errorf("no queue for placement %v", req.Placement)
	}
	removedSegments := int32(q.Clean(req.UpdatedBefore))
	return &pb.JobQueueCleanResponse{
		RemovedSegments: removedSegments,
	}, nil
}

func (se *JobqEndpoint) cleanAll(updatedBefore time.Time) (*pb.JobQueueCleanResponse, error) {
	removedSegments := int32(0)
	for _, q := range se.queues.GetAllQueues() {
		removedSegments += int32(q.Clean(updatedBefore))
	}
	return &pb.JobQueueCleanResponse{
		RemovedSegments: removedSegments,
	}, nil
}

// Trim removes all jobs from the queue with health greater than the given
// value. If the given placement is negative, all queues are trimmed.
func (se *JobqEndpoint) Trim(ctx context.Context, req *pb.JobQueueTrimRequest) (*pb.JobQueueTrimResponse, error) {
	// req.Placement < 0 is deprecated; use AllPlacements
	if req.Placement < 0 || req.AllPlacements {
		return se.trimAll(req.HealthGreaterThan)
	}
	q := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if q == nil {
		return nil, fmt.Errorf("no queue for placement %v", req.Placement)
	}
	removedSegments := q.Trim(req.HealthGreaterThan)
	return &pb.JobQueueTrimResponse{
		RemovedSegments: int32(removedSegments),
	}, nil
}

func (se *JobqEndpoint) trimAll(healthGreaterThan float64) (*pb.JobQueueTrimResponse, error) {
	removedSegments := int32(0)
	for _, q := range se.queues.GetAllQueues() {
		removedSegments += int32(q.Trim(healthGreaterThan))
	}
	return &pb.JobQueueTrimResponse{
		RemovedSegments: removedSegments,
	}, nil
}

// NewEndpoint creates a new endpoint.
func NewEndpoint(log *zap.Logger, queues *QueueMap) *JobqEndpoint {
	return &JobqEndpoint{
		log:    log,
		queues: queues,
	}
}
