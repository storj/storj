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

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/irreparable"
	"storj.io/storj/satellite/repair/queue"
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
	RepairOverride            int           `help:"override value for repair threshold" default:"0"`
}

// durabilityStats remote segment information
type durabilityStats struct {
	objectsChecked              int64
	remoteSegmentsChecked       int64
	remoteSegmentsNeedingRepair int64
	remoteSegmentsLost          int64
	remoteSegmentInfo           []string
}

// Checker contains the information needed to do checks for missing pieces
//
// architecture: Chore
type Checker struct {
	logger          *zap.Logger
	repairQueue     queue.RepairQueue
	irrdb           irreparable.DB
	metainfo        *metainfo.Service
	metaLoop        *metainfo.Loop
	nodestate       *ReliabilityCache
	repairOverride  int32
	Loop            sync2.Cycle
	IrreparableLoop sync2.Cycle
}

// NewChecker creates a new instance of checker
func NewChecker(logger *zap.Logger, repairQueue queue.RepairQueue, irrdb irreparable.DB, metainfo *metainfo.Service, metaLoop *metainfo.Loop, overlay *overlay.Service, config Config) *Checker {
	return &Checker{
		logger: logger,

		repairQueue:    repairQueue,
		irrdb:          irrdb,
		metainfo:       metainfo,
		metaLoop:       metaLoop,
		nodestate:      NewReliabilityCache(overlay, config.ReliabilityCacheStaleness),
		repairOverride: int32(config.RepairOverride),

		Loop:            *sync2.NewCycle(config.Interval),
		IrreparableLoop: *sync2.NewCycle(config.IrreparableInterval),
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

// IdentifyInjuredSegments checks for missing pieces off of the metainfo and overlay.
func (checker *Checker) IdentifyInjuredSegments(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	observer := &checkerObserver{
		repairQueue:    checker.repairQueue,
		irrdb:          checker.irrdb,
		nodestate:      checker.nodestate,
		monStats:       durabilityStats{},
		overrideRepair: checker.repairOverride,
		log:            checker.logger,
	}
	err = checker.metaLoop.Join(ctx, observer)
	if err != nil {
		if !errs2.IsCanceled(err) {
			checker.logger.Error("IdentifyInjuredSegments error", zap.Error(err))
		}
		return err
	}

	mon.IntVal("remote_files_checked").Observe(observer.monStats.objectsChecked)                        //locked
	mon.IntVal("remote_segments_checked").Observe(observer.monStats.remoteSegmentsChecked)              //locked
	mon.IntVal("remote_segments_needing_repair").Observe(observer.monStats.remoteSegmentsNeedingRepair) //locked
	mon.IntVal("remote_segments_lost").Observe(observer.monStats.remoteSegmentsLost)                    //locked
	mon.IntVal("remote_files_lost").Observe(int64(len(observer.monStats.remoteSegmentInfo)))            //locked

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
	// TODO figure out how to reduce duplicate code between here and checkerObs.RemoteSegment
	defer mon.Task()(&ctx)(&err)
	remote := pointer.GetRemote()
	if pointer.GetType() == pb.Pointer_INLINE || remote == nil {
		return nil
	}

	pieces := remote.GetRemotePieces()
	if pieces == nil {
		checker.logger.Debug("no pieces on remote segment")
		return nil
	}

	missingPieces, err := checker.nodestate.MissingPieces(ctx, pointer.CreationDate, pieces)
	if err != nil {
		return errs.Combine(Error.New("error getting missing pieces"), err)
	}

	numHealthy := int32(len(pieces) - len(missingPieces))
	redundancy := pointer.Remote.Redundancy

	// we repair when the number of healthy pieces is less than or equal to the repair threshold and is greater or equal to
	// minimum required pieces in redundancy
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	if numHealthy >= redundancy.MinReq && numHealthy <= redundancy.RepairThreshold && numHealthy < redundancy.SuccessThreshold {
		err = checker.repairQueue.Insert(ctx, &pb.InjuredSegment{
			Path:         []byte(path),
			LostPieces:   missingPieces,
			InsertedTime: time.Now().UTC(),
		})
		if err != nil {
			return errs.Combine(Error.New("error adding injured segment to queue"), err)
		}

		// delete always returns nil when something was deleted and also when element didn't exists
		err = checker.irrdb.Delete(ctx, []byte(path))
		if err != nil {
			checker.logger.Error("error deleting entry from irreparable db: ", zap.Error(err))
		}
	} else if numHealthy < redundancy.MinReq && numHealthy < redundancy.RepairThreshold {

		// make an entry into the irreparable table
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
			return errs.Combine(Error.New("error handling irreparable segment to queue"), err)
		}
	}
	return nil
}

var _ metainfo.Observer = (*checkerObserver)(nil)

// checkerObserver implements the metainfo loop Observer interface
//
// architecture: Observer
type checkerObserver struct {
	repairQueue    queue.RepairQueue
	irrdb          irreparable.DB
	nodestate      *ReliabilityCache
	monStats       durabilityStats
	overrideRepair int32
	log            *zap.Logger
}

func (obs *checkerObserver) RemoteSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
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
		return errs.Combine(Error.New("error getting missing pieces"), err)
	}

	numHealthy := int32(len(pieces) - len(missingPieces))
	mon.IntVal("checker_segment_total_count").Observe(int64(len(pieces)))  //locked
	mon.IntVal("checker_segment_healthy_count").Observe(int64(numHealthy)) //locked

	segmentAge := time.Since(pointer.CreationDate)
	mon.IntVal("checker_segment_age").Observe(int64(segmentAge.Seconds())) //locked

	redundancy := pointer.Remote.Redundancy

	repairThreshold := redundancy.RepairThreshold
	if obs.overrideRepair != 0 {
		repairThreshold = obs.overrideRepair
	}

	// we repair when the number of healthy pieces is less than or equal to the repair threshold and is greater or equal to
	// minimum required pieces in redundancy
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	if numHealthy >= redundancy.MinReq && numHealthy <= repairThreshold && numHealthy < redundancy.SuccessThreshold {
		obs.monStats.remoteSegmentsNeedingRepair++
		err = obs.repairQueue.Insert(ctx, &pb.InjuredSegment{
			Path:         []byte(path.Raw),
			LostPieces:   missingPieces,
			InsertedTime: time.Now().UTC(),
		})
		if err != nil {
			obs.log.Error("error adding injured segment to queue", zap.Error(err))
			return nil
		}

		// delete always returns nil when something was deleted and also when element didn't exists
		err = obs.irrdb.Delete(ctx, []byte(path.Raw))
		if err != nil {
			obs.log.Error("error deleting entry from irreparable db", zap.Error(err))
			return nil
		}
	} else if numHealthy < redundancy.MinReq && numHealthy < redundancy.RepairThreshold {
		// TODO: see whether this can be handled with metainfo.ScopedPath
		pathElements := storj.SplitPath(path.Raw)

		// check to make sure there are at least *4* path elements. the first three
		// are project, segment, and bucket name, but we want to make sure we're talking
		// about an actual object, and that there's an object name specified
		if len(pathElements) >= 4 {
			project, bucketName, segmentpath := pathElements[0], pathElements[2], pathElements[3]

			// TODO: is this correct? split splits all path components, but it's only using the third.
			lostSegInfo := storj.JoinPaths(project, bucketName, segmentpath)
			if !contains(obs.monStats.remoteSegmentInfo, lostSegInfo) {
				obs.monStats.remoteSegmentInfo = append(obs.monStats.remoteSegmentInfo, lostSegInfo)
			}
		}

		var segmentAge time.Duration
		if pointer.CreationDate.Before(pointer.LastRepaired) {
			segmentAge = time.Since(pointer.LastRepaired)
		} else {
			segmentAge = time.Since(pointer.CreationDate)
		}
		mon.IntVal("checker_segment_time_until_irreparable").Observe(int64(segmentAge.Seconds())) //locked

		obs.monStats.remoteSegmentsLost++
		// make an entry into the irreparable table
		segmentInfo := &pb.IrreparableSegment{
			Path:               []byte(path.Raw),
			SegmentDetail:      pointer,
			LostPieces:         int32(len(missingPieces)),
			LastRepairAttempt:  time.Now().Unix(),
			RepairAttemptCount: int64(1),
		}

		// add the entry if new or update attempt count if already exists
		err := obs.irrdb.IncrementRepairAttempts(ctx, segmentInfo)
		if err != nil {
			obs.log.Error("error handling irreparable segment to queue", zap.Error(err))
			return nil
		}
	}

	return nil
}

func (obs *checkerObserver) Object(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	obs.monStats.objectsChecked++

	return nil
}

func (obs *checkerObserver) InlineSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
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
			return errs.Combine(Error.New("error reading segment from the queue"), err)
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
