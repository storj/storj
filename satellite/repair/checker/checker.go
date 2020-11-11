// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/irreparable"
	"storj.io/storj/satellite/repair/queue"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("checker error")
	mon   = monkit.Package()
)

// Config contains configurable values for checker.
type Config struct {
	Interval            time.Duration `help:"how frequently checker should check for bad segments" releaseDefault:"30s" devDefault:"0h0m10s"`
	IrreparableInterval time.Duration `help:"how frequently irrepairable checker should check for lost pieces" releaseDefault:"30m" devDefault:"0h0m5s"`

	ReliabilityCacheStaleness time.Duration `help:"how stale reliable node cache can be" releaseDefault:"5m" devDefault:"5m"`
	RepairOverride            int           `help:"override value for repair threshold" releaseDefault:"52" devDefault:"0"`
}

// durabilityStats remote segment information.
type durabilityStats struct {
	objectsChecked                 int64
	remoteSegmentsChecked          int64
	remoteSegmentsNeedingRepair    int64
	newRemoteSegmentsNeedingRepair int64
	remoteSegmentsLost             int64
	remoteSegmentsFailedToCheck    int64
	remoteSegmentInfo              []metabase.ObjectLocation
	// remoteSegmentsOverThreshold[0]=# of healthy=rt+1, remoteSegmentsOverThreshold[1]=# of healthy=rt+2, etc...
	remoteSegmentsOverThreshold [5]int64
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
	Loop            *sync2.Cycle
	IrreparableLoop *sync2.Cycle
}

// NewChecker creates a new instance of checker.
func NewChecker(logger *zap.Logger, repairQueue queue.RepairQueue, irrdb irreparable.DB, metainfo *metainfo.Service, metaLoop *metainfo.Loop, overlay *overlay.Service, config Config) *Checker {
	return &Checker{
		logger: logger,

		repairQueue:    repairQueue,
		irrdb:          irrdb,
		metainfo:       metainfo,
		metaLoop:       metaLoop,
		nodestate:      NewReliabilityCache(overlay, config.ReliabilityCacheStaleness),
		repairOverride: int32(config.RepairOverride),

		Loop:            sync2.NewCycle(config.Interval),
		IrreparableLoop: sync2.NewCycle(config.IrreparableInterval),
	}
}

// Run the checker loop.
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

// Close halts the Checker loop.
func (checker *Checker) Close() error {
	checker.Loop.Close()
	return nil
}

// IdentifyInjuredSegments checks for missing pieces off of the metainfo and overlay.
func (checker *Checker) IdentifyInjuredSegments(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	startTime := time.Now()

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

	// remove all segments which were not seen as unhealthy by this checker iteration
	healthyDeleted, err := checker.repairQueue.Clean(ctx, startTime)
	if err != nil {
		return Error.Wrap(err)
	}

	mon.IntVal("remote_files_checked").Observe(observer.monStats.objectsChecked)                               //mon:locked
	mon.IntVal("remote_segments_checked").Observe(observer.monStats.remoteSegmentsChecked)                     //mon:locked
	mon.IntVal("remote_segments_failed_to_check").Observe(observer.monStats.remoteSegmentsFailedToCheck)       //mon:locked
	mon.IntVal("remote_segments_needing_repair").Observe(observer.monStats.remoteSegmentsNeedingRepair)        //mon:locked
	mon.IntVal("new_remote_segments_needing_repair").Observe(observer.monStats.newRemoteSegmentsNeedingRepair) //mon:locked
	mon.IntVal("remote_segments_lost").Observe(observer.monStats.remoteSegmentsLost)                           //mon:locked
	mon.IntVal("remote_files_lost").Observe(int64(len(observer.monStats.remoteSegmentInfo)))                   //mon:locked
	mon.IntVal("remote_segments_over_threshold_1").Observe(observer.monStats.remoteSegmentsOverThreshold[0])   //mon:locked
	mon.IntVal("remote_segments_over_threshold_2").Observe(observer.monStats.remoteSegmentsOverThreshold[1])   //mon:locked
	mon.IntVal("remote_segments_over_threshold_3").Observe(observer.monStats.remoteSegmentsOverThreshold[2])   //mon:locked
	mon.IntVal("remote_segments_over_threshold_4").Observe(observer.monStats.remoteSegmentsOverThreshold[3])   //mon:locked
	mon.IntVal("remote_segments_over_threshold_5").Observe(observer.monStats.remoteSegmentsOverThreshold[4])   //mon:locked
	mon.IntVal("healthy_segments_removed_from_queue").Observe(healthyDeleted)                                  //mon:locked

	allUnhealthy := observer.monStats.remoteSegmentsNeedingRepair + observer.monStats.remoteSegmentsFailedToCheck
	allChecked := observer.monStats.remoteSegmentsChecked
	allHealthy := allChecked - allUnhealthy
	mon.FloatVal("remote_segments_healthy_percentage").Observe(100 * float64(allHealthy) / float64(allChecked)) //mon:locked

	return nil
}

// checks for a object location in slice.
func containsObjectLocation(a []metabase.ObjectLocation, x metabase.ObjectLocation) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

func (checker *Checker) updateIrreparableSegmentStatus(ctx context.Context, pointer *pb.Pointer, key metabase.SegmentKey) (err error) {
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

	repairThreshold := redundancy.RepairThreshold
	if checker.repairOverride != 0 {
		repairThreshold = checker.repairOverride
	}

	// we repair when the number of healthy pieces is less than or equal to the repair threshold and is greater or equal to
	// minimum required pieces in redundancy
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	//
	// If the segment is suddenly entirely healthy again, we don't need to repair and we don't need to
	// keep it in the irreparabledb queue either.
	if numHealthy >= redundancy.MinReq && numHealthy <= repairThreshold && numHealthy < redundancy.SuccessThreshold {
		_, err = checker.repairQueue.Insert(ctx, &internalpb.InjuredSegment{
			Path:         key,
			LostPieces:   missingPieces,
			InsertedTime: time.Now().UTC(),
		}, int(numHealthy))
		if err != nil {
			return errs.Combine(Error.New("error adding injured segment to queue"), err)
		}

		// delete always returns nil when something was deleted and also when element didn't exists
		err = checker.irrdb.Delete(ctx, key)
		if err != nil {
			checker.logger.Error("error deleting entry from irreparable db: ", zap.Error(err))
		}
	} else if numHealthy < redundancy.MinReq && numHealthy < repairThreshold {

		// make an entry into the irreparable table
		segmentInfo := &internalpb.IrreparableSegment{
			Path:               key,
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
	} else if numHealthy > repairThreshold || numHealthy >= redundancy.SuccessThreshold {
		err = checker.irrdb.Delete(ctx, key)
		if err != nil {
			return Error.New("error removing segment from irreparable queue: %v", err)
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

func (obs *checkerObserver) RemoteSegment(ctx context.Context, segment *metainfo.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	// ignore pointer if expired
	if segment.Expired(time.Now()) {
		return nil
	}

	obs.monStats.remoteSegmentsChecked++

	// ensure we get values, even if only zero values, so that redash can have an alert based on this
	mon.Counter("checker_segments_below_min_req").Inc(0) //mon:locked

	pieces := segment.Pieces
	if len(pieces) == 0 {
		obs.log.Debug("no pieces on remote segment")
		return nil
	}

	pbPieces := make([]*pb.RemotePiece, len(pieces))
	for i, piece := range pieces {
		pbPieces[i] = &pb.RemotePiece{
			PieceNum: int32(piece.Number),
			NodeId:   piece.StorageNode,
		}
	}

	// TODO: update MissingPieces to accept metabase.Pieces
	missingPieces, err := obs.nodestate.MissingPieces(ctx, segment.CreationDate, pbPieces)
	if err != nil {
		obs.monStats.remoteSegmentsFailedToCheck++
		return errs.Combine(Error.New("error getting missing pieces"), err)
	}

	numHealthy := len(pieces) - len(missingPieces)
	mon.IntVal("checker_segment_total_count").Observe(int64(len(pieces)))  //mon:locked
	mon.IntVal("checker_segment_healthy_count").Observe(int64(numHealthy)) //mon:locked

	segmentAge := time.Since(segment.CreationDate)
	mon.IntVal("checker_segment_age").Observe(int64(segmentAge.Seconds())) //mon:locked

	required := int(segment.Redundancy.RequiredShares)
	repairThreshold := int(segment.Redundancy.RepairShares)
	if obs.overrideRepair != 0 {
		repairThreshold = int(obs.overrideRepair)
	}
	successThreshold := int(segment.Redundancy.OptimalShares)

	key := segment.Location.Encode()
	// we repair when the number of healthy pieces is less than or equal to the repair threshold and is greater or equal to
	// minimum required pieces in redundancy
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	if numHealthy >= required && numHealthy <= repairThreshold && numHealthy < successThreshold {
		obs.monStats.remoteSegmentsNeedingRepair++
		alreadyInserted, err := obs.repairQueue.Insert(ctx, &internalpb.InjuredSegment{
			Path:         key,
			LostPieces:   missingPieces,
			InsertedTime: time.Now().UTC(),
		}, numHealthy)
		if err != nil {
			obs.log.Error("error adding injured segment to queue", zap.Error(err))
			return nil
		}

		if !alreadyInserted {
			obs.monStats.newRemoteSegmentsNeedingRepair++
		}

		// delete always returns nil when something was deleted and also when element didn't exists
		err = obs.irrdb.Delete(ctx, key)
		if err != nil {
			obs.log.Error("error deleting entry from irreparable db", zap.Error(err))
			return nil
		}
	} else if numHealthy < required && numHealthy < repairThreshold {
		lostSegInfo := segment.Location.Object()
		if !containsObjectLocation(obs.monStats.remoteSegmentInfo, lostSegInfo) {
			obs.monStats.remoteSegmentInfo = append(obs.monStats.remoteSegmentInfo, lostSegInfo)
		}

		var segmentAge time.Duration
		if segment.CreationDate.Before(segment.LastRepaired) {
			segmentAge = time.Since(segment.LastRepaired)
		} else {
			segmentAge = time.Since(segment.CreationDate)
		}
		mon.IntVal("checker_segment_time_until_irreparable").Observe(int64(segmentAge.Seconds())) //mon:locked

		obs.monStats.remoteSegmentsLost++
		mon.Counter("checker_segments_below_min_req").Inc(1) //mon:locked
		// make an entry into the irreparable table
		segmentInfo := &internalpb.IrreparableSegment{
			Path:               key,
			SegmentDetail:      segment.Pointer, // TODO: replace with something better than pb.Pointer
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
	} else if numHealthy > repairThreshold && numHealthy <= (repairThreshold+len(obs.monStats.remoteSegmentsOverThreshold)) {
		// record metrics for segments right above repair threshold
		// numHealthy=repairThreshold+1 through numHealthy=repairThreshold+5
		for i := range obs.monStats.remoteSegmentsOverThreshold {
			if numHealthy == (repairThreshold + i + 1) {
				obs.monStats.remoteSegmentsOverThreshold[i]++
				break
			}
		}
	}

	return nil
}

func (obs *checkerObserver) Object(ctx context.Context, object *metainfo.Object) (err error) {
	defer mon.Task()(&ctx)(&err)

	obs.monStats.objectsChecked++

	return nil
}

func (obs *checkerObserver) InlineSegment(ctx context.Context, segment *metainfo.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

// IrreparableProcess iterates over all items in the irreparabledb. If an item can
// now be repaired then it is added to a worker queue.
func (checker *Checker) IrreparableProcess(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	const limit = 1000
	lastSeenSegmentKey := metabase.SegmentKey{}

	for {
		segments, err := checker.irrdb.GetLimited(ctx, limit, lastSeenSegmentKey)
		if err != nil {
			return errs.Combine(Error.New("error reading segment from the queue"), err)
		}

		// zero segments returned with nil err
		if len(segments) == 0 {
			break
		}

		lastSeenSegmentKey = metabase.SegmentKey(segments[len(segments)-1].Path)

		for _, segment := range segments {
			err = checker.updateIrreparableSegmentStatus(ctx, segment.GetSegmentDetail(), metabase.SegmentKey(segment.GetPath()))
			if err != nil {
				checker.logger.Error("irrepair segment checker failed: ", zap.Error(err))
			}
		}
	}

	return nil
}
