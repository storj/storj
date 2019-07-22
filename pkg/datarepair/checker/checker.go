// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("checker error")
	mon   = monkit.Package()
)

// Config contains configurable values for checker
type Config struct {
	Interval            time.Duration `help:"how frequently checker should check for bad segments" releaseDefault:"30s" devDefault:"0h0m10s"`
	IrreparableInterval time.Duration `help:"how frequently irrepairable checker should check for lost pieces" releaseDefault:"30m" devDefault:"0h0m5s"`

	ReliabilityCacheStaleness time.Duration `help:"how stale reliable node cache can be" releaseDefault:"5m" devDefault:"5m"`
}

// durabilityStats remote segment information
type durabilityStats struct {
	remoteFilesChecked          int64
	remoteSegmentsChecked       int64
	remoteSegmentsNeedingRepair int64
	remoteSegmentsLost          int64
	remoteSegmentInfo           []string
}

// Checker contains the information needed to do checks for missing pieces
type Checker struct {
	metainfo        *metainfo.Service
	lastChecked     string
	repairQueue     queue.RepairQueue
	nodestate       *ReliabilityCache
	irrdb           irreparable.DB
	logger          *zap.Logger
	Loop            sync2.Cycle
	IrreparableLoop sync2.Cycle
	metaLoop        *metainfo.Loop
}

// NewChecker creates a new instance of checker
func NewChecker(metainfo *metainfo.Service, repairQueue queue.RepairQueue, overlay *overlay.Cache, irrdb irreparable.DB, limit int, metaLoop *metainfo.Loop, logger *zap.Logger, config Config) *Checker {
	// TODO: reorder arguments
	return &Checker{
		metainfo:        metainfo,
		lastChecked:     "",
		repairQueue:     repairQueue,
		nodestate:       NewReliabilityCache(overlay, config.ReliabilityCacheStaleness),
		irrdb:           irrdb,
		logger:          logger,
		Loop:            *sync2.NewCycle(config.Interval),
		IrreparableLoop: *sync2.NewCycle(config.IrreparableInterval),
		metaLoop:        metaLoop,
	}
}

// Run the checker loop
func (checker *Checker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return checker.Loop.Run(ctx, checker.IdentifyInjuredSegments)
	})

	group.Go(func() error {
		return checker.IrreparableLoop.Run(ctx, checker.IrreparableProcess)
	})

	return group.Wait()
}

// RefreshReliabilityCache forces refreshing node online status cache.
func (checker *Checker) RefreshReliabilityCache(ctx context.Context) error {
	return checker.nodestate.Refresh(ctx)
}

// Close halts the Checker loop
func (checker *Checker) Close() error {
	checker.Loop.Close()
	return nil
}

// IdentifyInjuredSegments checks for missing pieces off of the metainfo and overlay cache
func (checker *Checker) IdentifyInjuredSegments(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	observer := &checkerObserver{
		repairQueue: checker.repairQueue,
		irrdb:       checker.irrdb,
		nodestate:   checker.nodestate,
		monStats:    durabilityStats{},
		log:         checker.logger,
	}
	err = checker.metaLoop.Join(ctx, observer)
	if err != nil {
		return Error.New("IdentifyInjuredSegments err %v", err)
	}

	mon.IntVal("remote_files_checked").Observe(observer.monStats.remoteFilesChecked)
	mon.IntVal("remote_segments_checked").Observe(observer.monStats.remoteSegmentsChecked)
	mon.IntVal("remote_segments_needing_repair").Observe(observer.monStats.remoteSegmentsNeedingRepair)
	mon.IntVal("remote_segments_lost").Observe(observer.monStats.remoteSegmentsLost)
	mon.IntVal("remote_files_lost").Observe(int64(len(observer.monStats.remoteSegmentInfo)))

	return nil
}

// checks for a string in slice
func contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

func (checker *Checker) updateIrreparableSegmentStatus(ctx context.Context, pointer *pb.Pointer, path string) (err error) {
	defer mon.Task()(&ctx)(&err)
	remote := pointer.GetRemote()
	if remote == nil {
		return nil
	}

	pieces := remote.GetRemotePieces()
	if pieces == nil {
		checker.logger.Debug("no pieces on remote segment")
		return nil
	}

	missingPieces, err := checker.nodestate.MissingPieces(ctx, pointer.CreationDate, pieces)
	if err != nil {
		return Error.New("error getting missing pieces %s", err)
	}

	numHealthy := int32(len(pieces) - len(missingPieces))
	redundancy := pointer.Remote.Redundancy

	// we repair when the number of healthy pieces is less than or equal to the repair threshold
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	if numHealthy > redundancy.MinReq && numHealthy <= redundancy.RepairThreshold && numHealthy < redundancy.SuccessThreshold {
		if len(missingPieces) == 0 {
			checker.logger.Warn("Missing pieces is zero in checker, but this should be impossible -- bad redundancy scheme.")
			return nil
		}
		err = checker.repairQueue.Insert(ctx, &pb.InjuredSegment{
			Path:         []byte(path),
			LostPieces:   missingPieces,
			InsertedTime: time.Now().UTC(),
		})
		if err != nil {
			return Error.New("error adding injured segment to queue %s", err)
		}

		// delete always returns nil when something was deleted and also when element didn't exists
		err = checker.irrdb.Delete(ctx, []byte(path))
		if err != nil {
			checker.logger.Error("error deleting entry from irreparable db: ", zap.Error(err))
		}
		// we need one additional piece for error correction. If only the minimum is remaining the file can't be repaired and is lost.
		// except for the case when minimum and repair thresholds are the same (a case usually seen during testing)
	} else if numHealthy <= redundancy.MinReq && numHealthy < redundancy.RepairThreshold {

		// make an entry in to the irreparable table
		segmentInfo := &pb.IrreparableSegment{
			Path:               []byte(path),
			SegmentDetail:      pointer,
			LostPieces:         int32(len(missingPieces)),
			LastRepairAttempt:  time.Now().Unix(),
			RepairAttemptCount: int64(1),
		}

		// add the entry if new or update attempt count if already exists
		err := checker.irrdb.IncrementRepairAttempts(ctx, segmentInfo)
		if err != nil {
			return Error.New("error handling irreparable segment to queue %s", err)
		}
	}
	return nil
}

// checkerObserver implements the metainfo loop Observer interface
type checkerObserver struct {
	repairQueue queue.RepairQueue
	irrdb       irreparable.DB
	nodestate   *ReliabilityCache
	monStats    durabilityStats
	log         *zap.Logger
}

func (obs *checkerObserver) RemoteSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	obs.monStats.remoteSegmentsChecked++
	remote := pointer.GetRemote()

	pieces := remote.GetRemotePieces()
	if pieces == nil {
		obs.log.Debug("no pieces on remote segment")
		return nil
	}

	missingPieces, err := obs.nodestate.MissingPieces(ctx, pointer.CreationDate, pieces)
	if err != nil {
		return Error.New("error getting missing pieces %s", err)
	}

	numHealthy := int32(len(pieces) - len(missingPieces))
	redundancy := pointer.Remote.Redundancy
	mon.IntVal("checker_segment_total_count").Observe(int64(len(pieces)))
	mon.IntVal("checker_segment_healthy_count").Observe(int64(numHealthy))

	// we repair when the number of healthy pieces is less than or equal to the repair threshold
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	if numHealthy > redundancy.MinReq && numHealthy <= redundancy.RepairThreshold && numHealthy < redundancy.SuccessThreshold {
		if len(missingPieces) == 0 {
			obs.log.Warn("Missing pieces is zero in checker, but this should be impossible -- bad redundancy scheme.")
			return nil
		}
		obs.monStats.remoteSegmentsNeedingRepair++
		err = obs.repairQueue.Insert(ctx, &pb.InjuredSegment{
			Path:         []byte(path),
			LostPieces:   missingPieces,
			InsertedTime: time.Now().UTC(),
		})
		if err != nil {
			obs.log.Sugar().Errorf("error adding injured segment to queue %s", err)
			return nil
		}

		// delete always returns nil when something was deleted and also when element didn't exists
		err = obs.irrdb.Delete(ctx, []byte(path))
		if err != nil {
			obs.log.Sugar().Errorf("error deleting entry from irreparable db: ", zap.Error(err))
			return nil
		}
		// we need one additional piece for error correction. If only the minimum is remaining the file can't be repaired and is lost.
		// except for the case when minimum and repair thresholds are the same (a case usually seen during testing)
	} else if numHealthy <= redundancy.MinReq && numHealthy < redundancy.RepairThreshold {
		pathElements := storj.SplitPath(path)

		// check to make sure there are at least *4* path elements. the first three
		// are project, segment, and bucket name, but we want to make sure we're talking
		// about an actual object, and that there's an object name specified
		if len(pathElements) >= 4 {
			project, bucketName, segmentpath := pathElements[0], pathElements[2], pathElements[3]
			lostSegInfo := storj.JoinPaths(project, bucketName, segmentpath)
			if contains(obs.monStats.remoteSegmentInfo, lostSegInfo) == false {
				obs.monStats.remoteSegmentInfo = append(obs.monStats.remoteSegmentInfo, lostSegInfo)
			}
		}

		obs.monStats.remoteSegmentsLost++
		// make an entry in to the irreparable table
		segmentInfo := &pb.IrreparableSegment{
			Path:               []byte(path),
			SegmentDetail:      pointer,
			LostPieces:         int32(len(missingPieces)),
			LastRepairAttempt:  time.Now().Unix(),
			RepairAttemptCount: int64(1),
		}

		// add the entry if new or update attempt count if already exists
		err := obs.irrdb.IncrementRepairAttempts(ctx, segmentInfo)
		if err != nil {
			obs.log.Sugar().Errorf("error handling irreparable segment to queue %s", err)
			return nil
		}
	}

	return nil
}

func (obs *checkerObserver) RemoteObject(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	obs.monStats.remoteFilesChecked++

	return nil
}

func (obs *checkerObserver) InlineSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

// IrreparableProcess iterates over all items in the irreparabledb. If an item can
// now be repaired then it is added to a worker queue.
func (checker *Checker) IrreparableProcess(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	const limit = 1000
	lastSeenSegmentPath := []byte{}

	for {
		segments, err := checker.irrdb.GetLimited(ctx, limit, lastSeenSegmentPath)
		if err != nil {
			return Error.New("error reading segment from the queue %s", err)
		}

		// zero segments returned with nil err
		if len(segments) == 0 {
			break
		}

		lastSeenSegmentPath = segments[len(segments)-1].Path

		for _, segment := range segments {
			err = checker.updateIrreparableSegmentStatus(ctx, segment.GetSegmentDetail(), string(segment.GetPath()))
			if err != nil {
				checker.logger.Error("irrepair segment checker failed: ", zap.Error(err))
			}
		}
	}

	return nil
}
