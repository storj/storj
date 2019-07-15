// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// durabilityStats remote segment information
type durabilityStats struct {
	remoteFilesChecked          int64
	remoteSegmentsChecked       int64
	remoteSegmentsNeedingRepair int64
	remoteSegmentsLost          int64
	remoteSegmentInfo           []string
}

// Observer is an observer that subscribes to the metainfo loop
type Observer struct {
	monStats    durabilityStats
	nodestate   *ReliabilityCache
	log         *zap.Logger
	repairQueue queue.RepairQueue
	irrdb       irreparable.DB
}

// NewObserver returns a new checker observer
func (checker *Checker) NewObserver() *Observer {
	return &Observer{
		log:         checker.log.Named("checker observer"),
		monStats:    durabilityStats{},
		nodestate:   checker.nodestate,
		repairQueue: checker.repairQueue,
		irrdb:       checker.irrdb,
	}
}

// RemoteSegment is called when the metainfo loop iterates over a remote segment
func (checkerObserver *Observer) RemoteSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	pieces := pointer.GetRemote().GetRemotePieces()
	if pieces == nil {
		checkerObserver.log.Debug("no pieces on remote segment")
	}

	missingPieces, err := checkerObserver.nodestate.MissingPieces(ctx, pointer.CreationDate, pieces)
	if err != nil {
		return Error.New("error getting missing pieces %s", err)
	}

	checkerObserver.monStats.remoteSegmentsChecked++

	numHealthy := int32(len(pieces) - len(missingPieces))
	redundancy := pointer.Remote.Redundancy
	mon.IntVal("checker_segment_total_count").Observe(int64(len(pieces)))
	mon.IntVal("checker_segment_healthy_count").Observe(int64(numHealthy))

	// we repair when the number of healthy pieces is less than or equal to the repair threshold
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	if numHealthy > redundancy.MinReq && numHealthy <= redundancy.RepairThreshold && numHealthy < redundancy.SuccessThreshold {
		if len(missingPieces) == 0 {
			checkerObserver.log.Warn("Missing pieces is zero in checker, but this should be impossible -- bad redundancy scheme.")
			return nil
		}
		checkerObserver.monStats.remoteSegmentsNeedingRepair++
		err = checkerObserver.repairQueue.Insert(ctx, &pb.InjuredSegment{
			Path:         []byte(path),
			LostPieces:   missingPieces,
			InsertedTime: time.Now().UTC(),
		})
		if err != nil {
			return Error.New("error adding injured segment to queue %s", err)
		}

		// delete always returns nil when something was deleted and also when element didn't exists
		err = checkerObserver.irrdb.Delete(ctx, []byte(path))
		if err != nil {
			checkerObserver.log.Error("error deleting entry from irreparable db: ", zap.Error(err))
		}
		// we need one additional piece for error correction. If only the minimum is remaining the file can't be repaired and is lost.
		// except for the case when minimum and repair thresholds are the same (a case usually seen during testing)
	} else if numHealthy <= redundancy.MinReq && numHealthy < redundancy.RepairThreshold {
		// check to make sure there are at least *4* path elements. the first three
		// are project, segment, and bucket name, but we want to make sure we're talking
		// about an actual object, and that there's an object name specified
		pathElements := storj.SplitPath(path)
		if len(pathElements) >= 4 {
			project, bucketName, segmentpath := pathElements[0], pathElements[2], pathElements[3]
			lostSegInfo := storj.JoinPaths(project, bucketName, segmentpath)
			if contains(checkerObserver.monStats.remoteSegmentInfo, lostSegInfo) == false {
				checkerObserver.monStats.remoteSegmentInfo = append(checkerObserver.monStats.remoteSegmentInfo, lostSegInfo)
			}
		}

		checkerObserver.monStats.remoteSegmentsLost++
		// make an entry in to the irreparable table
		segmentInfo := &pb.IrreparableSegment{
			Path:               []byte(path),
			SegmentDetail:      pointer,
			LostPieces:         int32(len(missingPieces)),
			LastRepairAttempt:  time.Now().Unix(),
			RepairAttemptCount: int64(1),
		}

		// add the entry if new or update attempt count if already exists
		err := checkerObserver.irrdb.IncrementRepairAttempts(ctx, segmentInfo)
		if err != nil {
			return Error.New("error handling irreparable segment to queue %s", err)
		}
	}
	return nil
}

// RemoteObject is called when the metainfo loop iterates over the last remote segment of an object
func (checkerObserver *Observer) RemoteObject(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	checkerObserver.monStats.remoteFilesChecked++
	return nil
}

// InlineSegment is called when the metainfo loop iterates over an inline segment
func (checkerObserver *Observer) InlineSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}
