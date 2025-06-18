// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"errors"
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
func (se *JobqEndpoint) Push(ctx context.Context, req *pb.JobQueuePushRequest) (_ *pb.JobQueuePushResponse, err error) {
	mon.Task()(&ctx)(&err)

	reqJob := req.GetJob()
	if reqJob == nil {
		return nil, errors.New("missing job")
	}
	q, err := se.queues.GetQueue(storj.PlacementConstraint(reqJob.Placement))
	if err != nil {
		return nil, fmt.Errorf("failed to get queue for placement %d: %w", reqJob.Placement, err)
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
func (se *JobqEndpoint) PushBatch(ctx context.Context, req *pb.JobQueuePushBatchRequest) (_ *pb.JobQueuePushBatchResponse, err error) {
	mon.Task()(&ctx)(&err)

	encounteredErrors := []error{}
	nonNilErrors := false
	wasNewList := make([]bool, len(req.Jobs))

	for i, reqJob := range req.Jobs {
		q, err := se.queues.GetQueue(storj.PlacementConstraint(reqJob.Placement))
		if err != nil {
			encounteredErrors = append(encounteredErrors, fmt.Errorf("failed to get queue for placement %d: %w", reqJob.Placement, err))
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
		return nil, fmt.Errorf("could not push all jobs to queues: %w", errors.Join(encounteredErrors...))
	}
	return &pb.JobQueuePushBatchResponse{
		NewlyInserted: wasNewList,
	}, nil
}

// Pop removes and returns the 'limit' lowest-health jobs from the queues for
// the requested placements.
func (se *JobqEndpoint) Pop(ctx context.Context, req *pb.JobQueuePopRequest) (_ *pb.JobQueuePopResponse, err error) {
	mon.Task()(&ctx)(&err)

	// otherwise we need to check all requested queues for the lowest health match
	queues := se.queues.ChooseQueues(int32SliceToPlacementConstraints(req.IncludedPlacements), int32SliceToPlacementConstraints(req.ExcludedPlacements))
	jobs := jobqueue.PopNMultipleQueues(int(req.Limit), queues)
	pbJobs := make([]*pb.RepairJob, len(jobs))
	for i, j := range jobs {
		pbJobs[i] = jobq.ConvertJobToProtobuf(j)
	}
	return &pb.JobQueuePopResponse{Jobs: pbJobs}, nil
}

// Peek returns the lowest-health job from the queues for the requested
// placement without removing the job from its queue.
func (se *JobqEndpoint) Peek(ctx context.Context, req *pb.JobQueuePeekRequest) (_ *pb.JobQueuePeekResponse, err error) {
	mon.Task()(&ctx)(&err)

	queues := se.queues.ChooseQueues(int32SliceToPlacementConstraints(req.IncludedPlacements), int32SliceToPlacementConstraints(req.ExcludedPlacements))
	jobs := jobqueue.PeekNMultipleQueues(int(req.Limit), queues)
	pbJobs := make([]*pb.RepairJob, len(jobs))
	for i, j := range jobs {
		pbJobs[i] = jobq.ConvertJobToProtobuf(j)
	}
	return &pb.JobQueuePeekResponse{Jobs: pbJobs}, nil
}

// Len returns the number of jobs in the queues for the requested placement.
func (se *JobqEndpoint) Len(ctx context.Context, req *pb.JobQueueLengthRequest) (_ *pb.JobQueueLengthResponse, err error) {
	mon.Task()(&ctx)(&err)

	if req.AllPlacements {
		return se.lenAll(ctx)
	}
	q, err := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if err != nil {
		return nil, fmt.Errorf("failed to get queue for placement %d: %w", req.Placement, err)
	}
	repairLen, retryLen := q.Len()
	return &pb.JobQueueLengthResponse{RepairLength: repairLen, RetryLength: retryLen}, nil
}

func (se *JobqEndpoint) lenAll(ctx context.Context) (*pb.JobQueueLengthResponse, error) {
	var repairLen, retryLen int64
	for _, q := range se.queues.GetAllQueues() {
		repair, retry := q.Len()
		repairLen += repair
		retryLen += retry
	}
	return &pb.JobQueueLengthResponse{RepairLength: repairLen, RetryLength: retryLen}, nil
}

// Delete removes a specific job from the queue by its placement, streamID, and
// position.
func (se *JobqEndpoint) Delete(ctx context.Context, req *pb.JobQueueDeleteRequest) (_ *pb.JobQueueDeleteResponse, err error) {
	mon.Task()(&ctx)(&err)

	streamID, err := uuid.FromBytes(req.StreamId)
	if err != nil {
		return nil, fmt.Errorf("invalid stream id %x: %w", req.StreamId, err)
	}
	q, err := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if err != nil {
		return nil, fmt.Errorf("failed to get queue for placement %d: %w", req.Placement, err)
	}
	removed := q.Delete(streamID, req.Position)
	return &pb.JobQueueDeleteResponse{
		DidDelete: removed,
	}, nil
}

// Inspect finds a particular job in the queue by its placement, streamID, and
// position and returns all of the job information.
func (se *JobqEndpoint) Inspect(ctx context.Context, req *pb.JobQueueInspectRequest) (_ *pb.JobQueueInspectResponse, err error) {
	mon.Task()(&ctx)(&err)

	streamID, err := uuid.FromBytes(req.StreamId)
	if err != nil {
		return nil, fmt.Errorf("invalid stream id %x: %w", req.StreamId, err)
	}
	position := req.Position
	q, err := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if err != nil {
		return nil, fmt.Errorf("failed to get queue for placement %d: %w", req.Placement, err)
	}
	job, ok := q.Inspect(streamID, position)
	return &pb.JobQueueInspectResponse{
		Job:   jobq.ConvertJobToProtobuf(job),
		Found: ok,
	}, nil
}

// Truncate removes all jobs from the queue for the requested placement. The
// queue is not destroyed.
func (se *JobqEndpoint) Truncate(ctx context.Context, req *pb.JobQueueTruncateRequest) (_ *pb.JobQueueTruncateResponse, err error) {
	mon.Task()(&ctx)(&err)

	if req.AllPlacements {
		return se.truncateAll()
	}
	q, err := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if err != nil {
		return nil, fmt.Errorf("failed to get queue for placement %d: %w", req.Placement, err)
	}
	q.Truncate()
	return &pb.JobQueueTruncateResponse{}, nil
}

func (se *JobqEndpoint) truncateAll() (*pb.JobQueueTruncateResponse, error) {
	for _, q := range se.queues.GetAllQueues() {
		q.Truncate()
	}
	return &pb.JobQueueTruncateResponse{}, nil
}

// Stat returns statistics about the queues for the requested placement.
// Note: this is expensive! It requires a full scan of the target queues.
func (se *JobqEndpoint) Stat(ctx context.Context, req *pb.JobQueueStatRequest) (_ *pb.JobQueueStatResponse, err error) {
	mon.Task()(&ctx)(&err)

	if req.AllPlacements {
		return se.statAll(ctx, req.WithHistogram)
	}
	placement := storj.PlacementConstraint(req.Placement)
	q, err := se.queues.GetQueue(placement)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue for placement %d: %w", req.Placement, err)
	}
	repairStat, retryStat, err := q.Stat(ctx)
	if err != nil {
		return nil, err
	}
	rps := &pb.QueueStat{
		Placement:        req.Placement,
		Count:            repairStat.Count,
		MaxInsertedAt:    repairStat.MaxInsertedAt,
		MinInsertedAt:    repairStat.MinInsertedAt,
		MaxAttemptedAt:   nil, // see statAll for an explanation of why these are nil
		MinAttemptedAt:   nil,
		MinSegmentHealth: repairStat.MinSegmentHealth,
		MaxSegmentHealth: repairStat.MaxSegmentHealth,
	}
	rys := &pb.QueueStat{
		Placement:        req.Placement,
		Count:            retryStat.Count,
		MaxInsertedAt:    retryStat.MaxInsertedAt,
		MinInsertedAt:    retryStat.MinInsertedAt,
		MaxAttemptedAt:   retryStat.MaxAttemptedAt,
		MinAttemptedAt:   retryStat.MinAttemptedAt,
		MinSegmentHealth: retryStat.MinSegmentHealth,
		MaxSegmentHealth: retryStat.MaxSegmentHealth,
	}
	if req.WithHistogram {
		rps.Histogram = histToProto(repairStat.Histogram)
		rys.Histogram = histToProto(retryStat.Histogram)
	}

	return &pb.JobQueueStatResponse{
		Stats: []*pb.QueueStat{
			rps, rys,
		},
	}, nil
}

func histToProto(histogram []jobq.HistogramItem) (res []*pb.QueueStatHistogram) {
	for _, h := range histogram {
		res = append(res, &pb.QueueStatHistogram{
			Count:                    h.Count,
			NumOutOfPlacement:        int32(h.NumOutOfPlacement),
			NumNormalizedHealthy:     int32(h.NumNormalizedHealthy),
			NumNormalizedRetrievable: int32(h.NumNormalizedRetrievable),
			ExemplarStreamId:         h.Exemplar.StreamID[:],
			ExemplarPosition:         h.Exemplar.Position,
		})
	}
	return res
}

func (se *JobqEndpoint) statAll(ctx context.Context, histogram bool) (*pb.JobQueueStatResponse, error) {
	queues := se.queues.GetAllQueues()
	pbStats := make([]*pb.QueueStat, 0, len(queues))
	for placement, q := range queues {
		repairStat, retryStat, err := q.Stat(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not collect statistics from queue for placement %d: %w", placement, err)
		}
		// We must fudge this a little. Older repair code expects stats to be
		// grouped by placement and by whether AttemptedAt is set. Jobq groups
		// stats by placement and by whether the job is in the repair or retry
		// queue. We think that it will be close enough to the intent of the old
		// code if we show repairStats as having no AttemptedAt, and retryStats
		// as having it set. This isn't perfect (it is in fact possible for
		// repairStats to have AttemptedAt set), but it may be good enough.
		repair := &pb.QueueStat{
			Placement:        int32(placement),
			Count:            repairStat.Count,
			MaxInsertedAt:    repairStat.MaxInsertedAt,
			MinInsertedAt:    repairStat.MinInsertedAt,
			MaxAttemptedAt:   nil,
			MinAttemptedAt:   nil,
			MinSegmentHealth: repairStat.MinSegmentHealth,
			MaxSegmentHealth: repairStat.MaxSegmentHealth,
		}
		retry := &pb.QueueStat{
			Placement:        int32(placement),
			Count:            retryStat.Count,
			MaxInsertedAt:    retryStat.MaxInsertedAt,
			MinInsertedAt:    retryStat.MinInsertedAt,
			MaxAttemptedAt:   retryStat.MaxAttemptedAt,
			MinAttemptedAt:   retryStat.MinAttemptedAt,
			MinSegmentHealth: retryStat.MinSegmentHealth,
			MaxSegmentHealth: retryStat.MaxSegmentHealth,
		}
		if histogram {
			repair.Histogram = histToProto(repairStat.Histogram)
			retry.Histogram = histToProto(retryStat.Histogram)
		}
		pbStats = append(pbStats, repair, retry)

	}
	return &pb.JobQueueStatResponse{
		Stats: pbStats,
	}, nil
}

// Clean removes all jobs from the queue that were last updated before the
// requested time. If the given placement is negative, all queues are cleaned.
func (se *JobqEndpoint) Clean(ctx context.Context, req *pb.JobQueueCleanRequest) (_ *pb.JobQueueCleanResponse, err error) {
	mon.Task()(&ctx)(&err)

	// req.Placement < 0 is deprecated; use AllPlacements
	if req.Placement < 0 || req.AllPlacements {
		return se.cleanAll(req.UpdatedBefore)
	}
	q, err := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if err != nil {
		return nil, fmt.Errorf("failed to get queue for placement %d: %w", req.Placement, err)
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
func (se *JobqEndpoint) Trim(ctx context.Context, req *pb.JobQueueTrimRequest) (_ *pb.JobQueueTrimResponse, err error) {
	mon.Task()(&ctx)(&err)

	// req.Placement < 0 is deprecated; use AllPlacements
	if req.Placement < 0 || req.AllPlacements {
		return se.trimAll(req.HealthGreaterThan)
	}
	q, err := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if err != nil {
		return nil, fmt.Errorf("failed to get queue for placement %d: %w", req.Placement, err)
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

// TestingSetAttemptedTime sets the attempted time for a specific job in the
// queue. This is a testing-only method.
func (se *JobqEndpoint) TestingSetAttemptedTime(ctx context.Context, req *pb.JobQueueTestingSetAttemptedTimeRequest) (_ *pb.JobQueueTestingSetAttemptedTimeResponse, err error) {
	mon.Task()(&ctx)(&err)

	q, err := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if err != nil {
		return nil, fmt.Errorf("failed to get queue for placement %d: %w", req.Placement, err)
	}
	streamID, err := uuid.FromBytes(req.StreamId)
	if err != nil {
		return nil, fmt.Errorf("invalid stream id %x: %w", req.StreamId, err)
	}
	updated := q.TestingSetAttemptedTime(streamID, req.Position, req.NewTime)
	return &pb.JobQueueTestingSetAttemptedTimeResponse{
		RowsAffected: int32(updated),
	}, nil
}

// TestingSetUpdatedTime sets the updated time for a specific job in the
// queue. This is a testing-only method.
func (se *JobqEndpoint) TestingSetUpdatedTime(ctx context.Context, req *pb.JobQueueTestingSetUpdatedTimeRequest) (_ *pb.JobQueueTestingSetUpdatedTimeResponse, err error) {
	mon.Task()(&ctx)(&err)

	q, err := se.queues.GetQueue(storj.PlacementConstraint(req.Placement))
	if err != nil {
		return nil, fmt.Errorf("failed to get queue for placement %d: %w", req.Placement, err)
	}
	streamID, err := uuid.FromBytes(req.StreamId)
	if err != nil {
		return nil, fmt.Errorf("invalid stream id %x: %w", req.StreamId, err)
	}
	updated := q.TestingSetUpdatedTime(streamID, req.Position, req.NewTime)
	return &pb.JobQueueTestingSetUpdatedTimeResponse{
		RowsAffected: int32(updated),
	}, nil
}

// NewEndpoint creates a new endpoint.
func NewEndpoint(log *zap.Logger, queues *QueueMap) *JobqEndpoint {
	return &JobqEndpoint{
		log:    log,
		queues: queues,
	}
}

func int32SliceToPlacementConstraints(placements []int32) []storj.PlacementConstraint {
	slice := make([]storj.PlacementConstraint, len(placements))
	for i, p := range placements {
		slice[i] = storj.PlacementConstraint(p)
	}
	return slice
}
