// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobq

import (
	"fmt"
	"time"
	"unsafe"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	pb "storj.io/storj/satellite/internalpb"
)

// SegmentIdentifier identifies individual segments in the repair queue.
type SegmentIdentifier struct {
	// StreamID is the stream ID of the segment.
	StreamID uuid.UUID
	// Position is the position of the segment.
	Position uint64
}

// ServerTimeNow is a flag indicating that the server should fill in its idea
// of the current time for the indicated field. This must be a value that is
// preserved through a uint64->time.Time->uint64 conversion round trip and is
// unlikely to be used naturally.
const ServerTimeNow uint64 = 123456789 // 1973-11-29T21:33:09Z

// RepairJob represents the in-memory structure of a job in the queue. This
// structure does not _need_ to be a multiple of 64 bits in size, but it's
// probably going to be aligned to a multiple of 64 bits in memory either way,
// so it's more efficient if we use that whole space.
type RepairJob struct {
	ID                       SegmentIdentifier
	Health                   float64
	InsertedAt               uint64
	LastAttemptedAt          uint64
	UpdatedAt                uint64
	NumAttempts              uint16
	Placement                uint16
	NumNormalizedHealthy     int16
	NumNormalizedRetrievable int16
	NumOutOfPlacement        int16
}

// LastAttemptedAtTime returns the LastAttemptedAt field as a time.Time.
func (rj RepairJob) LastAttemptedAtTime() time.Time {
	if rj.LastAttemptedAt != 0 {
		return time.Unix(int64(rj.LastAttemptedAt), 0)
	}
	return time.Time{}
}

// RecordSize is the size of a RepairJob record in bytes. It includes any
// padding that may be added by the compiler to align the record to a multiple
// of the word size for the target arch.
var RecordSize = unsafe.Sizeof(RepairJob{})

// ConvertJobToProtobuf converts a RepairJob record to a protobuf
// representation.
func ConvertJobToProtobuf(job RepairJob) *pb.RepairJob {
	protoJob := &pb.RepairJob{
		StreamId:                 job.ID.StreamID[:],
		Position:                 job.ID.Position,
		Health:                   job.Health,
		NumAttempts:              int32(job.NumAttempts),
		Placement:                int32(job.Placement),
		NumNormalizedHealthy:     int32(job.NumNormalizedHealthy),
		NumNormalizedRetrievable: int32(job.NumNormalizedRetrievable),
		NumOutOfPlacement:        int32(job.NumOutOfPlacement),
	}
	if job.InsertedAt != 0 {
		insertedAt := time.Unix(int64(job.InsertedAt), 0)
		protoJob.InsertedAt = &insertedAt
	}
	if job.LastAttemptedAt != 0 {
		lastAttemptedAt := job.LastAttemptedAtTime()
		protoJob.LastAttemptedAt = &lastAttemptedAt
	}
	if job.UpdatedAt != 0 {
		updatedAt := time.Unix(int64(job.UpdatedAt), 0)
		protoJob.UpdatedAt = &updatedAt
	}
	return protoJob
}

// ConvertJobFromProtobuf converts a protobuf representation of a RepairJob to a
// RepairJob record.
func ConvertJobFromProtobuf(protoJob *pb.RepairJob) (RepairJob, error) {
	streamID, err := uuid.FromBytes(protoJob.StreamId)
	if err != nil {
		return RepairJob{}, fmt.Errorf("invalid stream id %x: %w", protoJob.StreamId, err)
	}

	job := RepairJob{
		ID: SegmentIdentifier{
			StreamID: streamID,
			Position: protoJob.Position,
		},
		Health:                   protoJob.Health,
		NumAttempts:              uint16(protoJob.NumAttempts),
		Placement:                uint16(protoJob.Placement),
		NumNormalizedHealthy:     int16(protoJob.NumNormalizedHealthy),
		NumNormalizedRetrievable: int16(protoJob.NumNormalizedRetrievable),
		NumOutOfPlacement:        int16(protoJob.NumOutOfPlacement),
	}

	var insertedAt time.Time
	if protoJob.InsertedAt != nil {
		insertedAt = *protoJob.InsertedAt
		insertedAtUnix := insertedAt.Unix()
		if insertedAtUnix > 0 {
			job.InsertedAt = uint64(insertedAtUnix)
		}
	}

	var lastAttemptedAt time.Time
	if protoJob.LastAttemptedAt != nil {
		lastAttemptedAt = *protoJob.LastAttemptedAt
	}
	lastAttemptedAtUnix := lastAttemptedAt.Unix()
	if lastAttemptedAtUnix > 0 {
		job.LastAttemptedAt = uint64(lastAttemptedAtUnix)
	}

	var updatedAt time.Time
	if protoJob.UpdatedAt != nil {
		updatedAt = *protoJob.UpdatedAt
	}
	updatedAtUnix := updatedAt.Unix()
	if updatedAtUnix > 0 {
		job.UpdatedAt = uint64(updatedAtUnix)
	}

	return job, nil
}

// QueueStat contains statistics about a queue or set of queues.
type QueueStat struct {
	Placement        storj.PlacementConstraint
	Count            int64
	MaxInsertedAt    time.Time
	MinInsertedAt    time.Time
	MaxAttemptedAt   *time.Time
	MinAttemptedAt   *time.Time
	MinSegmentHealth float64
	MaxSegmentHealth float64
	Histogram        []HistogramItem
}

// HistogramItem represents a group of jobq items with the same number of missing / out of placement health count.
type HistogramItem struct {
	NumNormalizedHealthy     int64
	NumNormalizedRetrievable int64
	NumOutOfPlacement        int64
	Exemplar                 SegmentIdentifier
	Count                    int64
}
