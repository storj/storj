// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobq

import (
	"context"
	"time"

	"storj.io/common/identity"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/repair/queue"
)

// This file relates to usage of jobq by storj.io/storj.

// Config holds the Storj-style configuration for a job queue client.
type Config struct {
	ServerNodeURL storj.NodeURL `help:"\"node URL\" of the job queue server" default:"" testDefault:""`
	TLS           tlsopts.Config
}

// RepairJobQueue is a Storj-style repair queue, meant to be a near drop-in
// replacement for the PostgreSQL arrangement.
type RepairJobQueue struct {
	jobqClient *Client
}

// Insert adds a segment to the appropriate repair queue. If the segment is
// already in the queue, it is not added again.  wasInserted is true if the
// segment was already added to the queue, false if it was inserted by this
// call.
func (rjq *RepairJobQueue) Insert(ctx context.Context, job *queue.InjuredSegment) (alreadyInserted bool, err error) {
	lastAttemptedAt := uint64(0)
	if job.AttemptedAt != nil {
		lastAttemptedAt = uint64(job.AttemptedAt.Unix())
	}

	wasNew, err := rjq.jobqClient.Push(ctx, RepairJob{
		ID:                       SegmentIdentifier{StreamID: job.StreamID, Position: job.Position.Encode()},
		Health:                   job.SegmentHealth,
		InsertedAt:               uint64(job.InsertedAt.Unix()),
		LastAttemptedAt:          lastAttemptedAt,
		UpdatedAt:                uint64(job.UpdatedAt.Unix()),
		Placement:                uint16(job.Placement),
		NumNormalizedHealthy:     job.NumNormalizedHealthy,
		NumNormalizedRetrievable: job.NumNormalizedRetrievable,
		NumOutOfPlacement:        job.NumOutOfPlacement,
	})
	// TODO: get and fill in numMissing and numOutOfPlacement
	return !wasNew, err
}

// InsertBatch adds multiple segments to the appropriate repair queues. If a
// segment is already in the queue, it is not added again.
// newlyInsertedSegments is the list of segments that were added to the queue.
func (rjq *RepairJobQueue) InsertBatch(ctx context.Context, segments []*queue.InjuredSegment) (newlyInsertedSegments []*queue.InjuredSegment, err error) {
	jobs := make([]RepairJob, len(segments))

	for i, segment := range segments {
		lastAttemptedAt := uint64(0)
		if segment.AttemptedAt != nil {
			lastAttemptedAt = uint64(segment.AttemptedAt.Unix())
		}
		jobs[i] = RepairJob{
			ID:                       SegmentIdentifier{StreamID: segment.StreamID, Position: segment.Position.Encode()},
			Health:                   segment.SegmentHealth,
			InsertedAt:               uint64(segment.InsertedAt.Unix()),
			LastAttemptedAt:          lastAttemptedAt,
			UpdatedAt:                uint64(segment.UpdatedAt.Unix()),
			Placement:                uint16(segment.Placement),
			NumNormalizedHealthy:     segment.NumNormalizedHealthy,
			NumNormalizedRetrievable: segment.NumNormalizedRetrievable,
			NumOutOfPlacement:        segment.NumOutOfPlacement,
		}
		// TODO: get and fill in numMissing and numOutOfPlacement
	}
	wasNewList, err := rjq.jobqClient.PushBatch(ctx, jobs)
	for i, wasNew := range wasNewList {
		if wasNew {
			newlyInsertedSegments = append(newlyInsertedSegments, segments[i])
		}
	}
	return newlyInsertedSegments, err
}

// Select removes and returns up to limit segments from the repair queue that
// match the given placement constraints. If includedPlacements is non-empty,
// only segments with placements in includedPlacements are returned. If
// excludedPlacements is non-empty, only segments with placements not in
// excludedPlacements are returned.
func (rjq *RepairJobQueue) Select(ctx context.Context, limit int, includedPlacements []storj.PlacementConstraint, excludedPlacements []storj.PlacementConstraint) ([]queue.InjuredSegment, error) {
	jobs, err := rjq.jobqClient.Pop(ctx, limit, includedPlacements, excludedPlacements)
	if err != nil {
		return nil, err
	}
	injuredSegments := make([]queue.InjuredSegment, 0, len(jobs))
	for _, job := range jobs {
		var attemptedAt *time.Time
		if job.LastAttemptedAt > 0 {
			t := time.Unix(int64(job.LastAttemptedAt), 0)
			attemptedAt = &t
		} else {
			// existing repair code expects filled-in AttemptedAt, even if this is
			// the first time the segment has been popped from the queue.
			t := time.Now()
			attemptedAt = &t
		}
		seg := queue.InjuredSegment{
			StreamID:                 job.ID.StreamID,
			Position:                 metabase.SegmentPositionFromEncoded(job.ID.Position),
			SegmentHealth:            job.Health,
			AttemptedAt:              attemptedAt,
			UpdatedAt:                time.Unix(int64(job.UpdatedAt), 0),
			InsertedAt:               time.Unix(int64(job.InsertedAt), 0),
			Placement:                storj.PlacementConstraint(job.Placement),
			NumNormalizedRetrievable: job.NumNormalizedRetrievable,
			NumNormalizedHealthy:     job.NumNormalizedHealthy,
			NumOutOfPlacement:        job.NumOutOfPlacement,
		}
		injuredSegments = append(injuredSegments, seg)
	}
	// existing repair tests depend on an error being returned here, instead of
	// checking the short count
	if len(jobs) == 0 {
		return nil, queue.ErrEmpty.New("")
	}
	return injuredSegments, nil
}

// Release does what's necessary to mark a repair job as succeeded or failed.
// In the case of RepairJobQueue, Release puts a segment back into the queue
// if it has failed.
func (rjq *RepairJobQueue) Release(ctx context.Context, job queue.InjuredSegment, repaired bool) error {
	if !repaired {
		// put the job back in the queue, mimicking how the segment
		// would be left in the queue under PostgreSQL/Cockroach,
		// just with an updated LastAttemptedAt.
		rJob := RepairJob{
			ID:                       SegmentIdentifier{StreamID: job.StreamID, Position: job.Position.Encode()},
			Health:                   job.SegmentHealth,
			InsertedAt:               uint64(job.InsertedAt.Unix()),
			LastAttemptedAt:          ServerTimeNow,
			UpdatedAt:                uint64(job.UpdatedAt.Unix()),
			Placement:                uint16(job.Placement),
			NumNormalizedHealthy:     job.NumNormalizedHealthy,
			NumNormalizedRetrievable: job.NumNormalizedRetrievable,
			NumOutOfPlacement:        job.NumOutOfPlacement,
		}
		rJob.NumAttempts++
		_, err := rjq.jobqClient.Push(ctx, rJob)
		return err
	}
	return nil
}

// SelectN returns up to limit segments from the repair queues- whichever
// segments from any queue have the lowest health, only including segments that
// are currently eligible for repair.
//
// Note that this is very different behavior from Select(); these segments are
// not removed from their queues. The similarity in naming is regrettable.
func (rjq *RepairJobQueue) SelectN(ctx context.Context, limit int) ([]queue.InjuredSegment, error) {
	segments, err := rjq.jobqClient.Peek(ctx, limit, nil, nil)
	if err != nil {
		return nil, err
	}
	injuredSegments := make([]queue.InjuredSegment, 0, len(segments))
	for _, job := range segments {
		var attemptedAt *time.Time
		if job.LastAttemptedAt > 0 {
			t := time.Unix(int64(job.LastAttemptedAt), 0)
			attemptedAt = &t
		}
		seg := queue.InjuredSegment{
			StreamID:                 job.ID.StreamID,
			Position:                 metabase.SegmentPositionFromEncoded(job.ID.Position),
			SegmentHealth:            job.Health,
			AttemptedAt:              attemptedAt,
			UpdatedAt:                time.Unix(int64(job.UpdatedAt), 0),
			InsertedAt:               time.Unix(int64(job.InsertedAt), 0),
			Placement:                storj.PlacementConstraint(job.Placement),
			NumNormalizedRetrievable: job.NumNormalizedRetrievable,
			NumNormalizedHealthy:     job.NumNormalizedHealthy,
			NumOutOfPlacement:        job.NumOutOfPlacement,
		}
		injuredSegments = append(injuredSegments, seg)
	}
	return injuredSegments, nil
}

// Clean removes all segments from the repair queue that were last updated
// before the given time. It returns the number of segments removed.
func (rjq *RepairJobQueue) Clean(ctx context.Context, updatedBefore time.Time) (int64, error) {
	numCleaned, err := rjq.jobqClient.CleanAll(ctx, updatedBefore)
	return int64(numCleaned), err
}

// Count returns the number of segments in the repair queues, including all
// placement queues and including retry queues.
func (rjq *RepairJobQueue) Count(ctx context.Context) (int, error) {
	repairLen, retryLen, err := rjq.jobqClient.LenAll(ctx)
	return int(repairLen + retryLen), err
}

// Delete removes a specific segment from its repair queue.
func (rjq *RepairJobQueue) Delete(ctx context.Context, job queue.InjuredSegment) error {
	// existing repair code expects no error when the segment was not found
	_, err := rjq.jobqClient.Delete(ctx, job.Placement, job.StreamID, job.Position.Encode())
	return err
}

// Stat returns statistics about the repair queues. Note: this is expensive!
// It requires a full scan of all queues.
func (rjq *RepairJobQueue) Stat(ctx context.Context) ([]queue.Stat, error) {
	stats, err := rjq.jobqClient.StatAll(ctx, false)
	if err != nil {
		return nil, err
	}
	queueStats := make([]queue.Stat, 0, len(stats))
	for _, stat := range stats {
		if stat.Count == 0 {
			continue
		}
		queueStats = append(queueStats, queue.Stat{
			Count:            int(stat.Count),
			Placement:        stat.Placement,
			MaxInsertedAt:    stat.MaxInsertedAt,
			MinInsertedAt:    stat.MinInsertedAt,
			MaxAttemptedAt:   stat.MaxAttemptedAt,
			MinAttemptedAt:   stat.MinAttemptedAt,
			MinSegmentHealth: stat.MinSegmentHealth,
			MaxSegmentHealth: stat.MaxSegmentHealth,
		})
	}
	return queueStats, nil
}

// TestingSetAttemptedTime is a testing-only method that sets the
// LastAttemptedAt field of a segment in the repair queue. It is not intended
// for production use.
func (rjq *RepairJobQueue) TestingSetAttemptedTime(ctx context.Context, placement storj.PlacementConstraint, streamID uuid.UUID, position metabase.SegmentPosition, t time.Time) (rowsAffected int64, err error) {
	return rjq.jobqClient.TestingSetAttemptedTime(ctx, placement, streamID, position.Encode(), t)
}

// TestingSetUpdatedTime is a testing-only method that sets the
// UpdatedAt field of a segment in the repair queue. It is not intended
// for production use.
func (rjq *RepairJobQueue) TestingSetUpdatedTime(ctx context.Context, placement storj.PlacementConstraint, streamID uuid.UUID, position metabase.SegmentPosition, t time.Time) (rowsAffected int64, err error) {
	return rjq.jobqClient.TestingSetUpdatedTime(ctx, placement, streamID, position.Encode(), t)
}

// Close closes the connection to the job queue server.
func (rjq *RepairJobQueue) Close() error {
	return rjq.jobqClient.Close()
}

var _ queue.RepairQueue = (*RepairJobQueue)(nil)

// WrapJobQueue wraps a jobq Client to become a RepairJobQueue.
func WrapJobQueue(cli *Client) *RepairJobQueue {
	return &RepairJobQueue{
		jobqClient: cli,
	}
}

// OpenJobQueue opens a RepairJobQueue with the given configuration.
func OpenJobQueue(ctx context.Context, fi *identity.FullIdentity, config Config) (*RepairJobQueue, error) {
	revocationDB, err := revocation.OpenDBFromCfg(ctx, config.TLS)
	if err != nil {
		return nil, err
	}
	if fi == nil {
		fi, err = identity.NewFullIdentity(ctx, identity.NewCAOptions{})
		if err != nil {
			return nil, err
		}
	}
	tlsOpts, err := tlsopts.NewOptions(fi, config.TLS, revocationDB)
	if err != nil {
		return nil, err
	}
	dialer := NewDialer(tlsOpts)
	rawConn, err := dialer.DialNodeURL(ctx, config.ServerNodeURL)
	if err != nil {
		return nil, err
	}

	conn := WrapConn(rawConn)
	return WrapJobQueue(conn), nil
}
