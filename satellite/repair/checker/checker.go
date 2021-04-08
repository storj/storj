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
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/metainfo/metaloop"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair"
	"storj.io/storj/satellite/repair/irreparable"
	"storj.io/storj/satellite/repair/queue"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("checker error")
	mon   = monkit.Package()
)

// Checker contains the information needed to do checks for missing pieces.
//
// architecture: Chore
type Checker struct {
	logger          *zap.Logger
	repairQueue     queue.RepairQueue
	irrdb           irreparable.DB
	metabase        metainfo.MetabaseDB
	metaLoop        *metaloop.Service
	nodestate       *ReliabilityCache
	statsCollector  *statsCollector
	repairOverrides RepairOverridesMap
	nodeFailureRate float64
	Loop            *sync2.Cycle
	IrreparableLoop *sync2.Cycle
}

// NewChecker creates a new instance of checker.
func NewChecker(logger *zap.Logger, repairQueue queue.RepairQueue, irrdb irreparable.DB, metabase metainfo.MetabaseDB, metaLoop *metaloop.Service, overlay *overlay.Service, config Config) *Checker {
	return &Checker{
		logger: logger,

		repairQueue:     repairQueue,
		irrdb:           irrdb,
		metabase:        metabase,
		metaLoop:        metaLoop,
		nodestate:       NewReliabilityCache(overlay, config.ReliabilityCacheStaleness),
		statsCollector:  newStatsCollector(),
		repairOverrides: config.RepairOverrides.GetMap(),
		nodeFailureRate: config.NodeFailureRate,

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

// getNodesEstimate updates the estimate of the total number of nodes. It is guaranteed
// to return a number greater than 0 when the error is nil.
//
// We can't calculate this upon first starting a Checker, because there may not be any
// nodes yet. We expect that there will be nodes before there are segments, though.
func (checker *Checker) getNodesEstimate(ctx context.Context) (int, error) {
	// this should be safe to call frequently; it is an efficient caching lookup.
	totalNumNodes, err := checker.nodestate.NumNodes(ctx)
	if err != nil {
		// We could proceed here by returning the last good value, or by returning a fallback
		// constant estimate, like "20000", and we'd probably be fine, but it would be better
		// not to have that happen silently for too long. Also, if we can't get this from the
		// database, we probably can't modify the injured segments queue, so it won't help to
		// proceed with this repair operation.
		return 0, err
	}
	if totalNumNodes == 0 {
		return 0, Error.New("segment health is meaningless: there are no nodes")
	}
	return totalNumNodes, nil
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
		repairQueue:      checker.repairQueue,
		irrdb:            checker.irrdb,
		nodestate:        checker.nodestate,
		statsCollector:   checker.statsCollector,
		monStats:         aggregateStats{},
		repairOverrides:  checker.repairOverrides,
		nodeFailureRate:  checker.nodeFailureRate,
		getNodesEstimate: checker.getNodesEstimate,
		log:              checker.logger,
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

	checker.statsCollector.collectAggregates()

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

func (checker *Checker) updateIrreparableSegmentStatus(ctx context.Context, key metabase.SegmentKey, redundancy storj.RedundancyScheme, creationDate time.Time, pieces metabase.Pieces) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(pieces) == 0 {
		checker.logger.Debug("no pieces on remote segment")
		return nil
	}

	missingPieces, err := checker.nodestate.MissingPieces(ctx, creationDate, pieces)
	if err != nil {
		return errs.Combine(Error.New("error getting missing pieces"), err)
	}

	numHealthy := int32(len(pieces) - len(missingPieces))

	repairThreshold := int32(redundancy.RepairShares)
	pbRedundancy := &pb.RedundancyScheme{
		MinReq:           int32(redundancy.RequiredShares),
		RepairThreshold:  int32(redundancy.RepairShares),
		SuccessThreshold: int32(redundancy.OptimalShares),
		Total:            int32(redundancy.TotalShares),
	}
	overrideValue := checker.repairOverrides.GetOverrideValuePB(pbRedundancy)
	if overrideValue != 0 {
		repairThreshold = overrideValue
	}

	totalNumNodes, err := checker.getNodesEstimate(ctx)
	if err != nil {
		return Error.New("could not get estimate of total number of nodes: %w", err)
	}

	// we repair when the number of healthy pieces is less than or equal to the repair threshold and is greater or equal to
	// minimum required pieces in redundancy
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	//
	// If the segment is suddenly entirely healthy again, we don't need to repair and we don't need to
	// keep it in the irreparabledb queue either.
	if numHealthy >= int32(redundancy.RequiredShares) && numHealthy <= repairThreshold && numHealthy < int32(redundancy.OptimalShares) {
		segmentHealth := repair.SegmentHealth(int(numHealthy), int(redundancy.RequiredShares), totalNumNodes, checker.nodeFailureRate)
		_, err = checker.repairQueue.Insert(ctx, &internalpb.InjuredSegment{
			Path:         key,
			LostPieces:   missingPieces,
			InsertedTime: time.Now().UTC(),
		}, segmentHealth)
		if err != nil {
			return errs.Combine(Error.New("error adding injured segment to queue"), err)
		}

		// delete always returns nil when something was deleted and also when element didn't exists
		err = checker.irrdb.Delete(ctx, key)
		if err != nil {
			checker.logger.Error("error deleting entry from irreparable db: ", zap.Error(err))
		}
	} else if numHealthy < int32(redundancy.RequiredShares) && numHealthy < repairThreshold {

		// make an entry into the irreparable table
		segmentInfo := &internalpb.IrreparableSegment{
			Path:               key,
			LostPieces:         int32(len(missingPieces)),
			LastRepairAttempt:  time.Now().Unix(),
			RepairAttemptCount: int64(1),
		}

		// add the entry if new or update attempt count if already exists
		err := checker.irrdb.IncrementRepairAttempts(ctx, segmentInfo)
		if err != nil {
			return errs.Combine(Error.New("error handling irreparable segment to queue"), err)
		}
	} else if numHealthy > repairThreshold || numHealthy >= int32(redundancy.OptimalShares) {
		err = checker.irrdb.Delete(ctx, key)
		if err != nil {
			return Error.New("error removing segment from irreparable queue: %v", err)
		}
	}
	return nil
}

var _ metaloop.Observer = (*checkerObserver)(nil)

// checkerObserver implements the metainfo loop Observer interface.
//
// architecture: Observer
type checkerObserver struct {
	repairQueue      queue.RepairQueue
	irrdb            irreparable.DB
	nodestate        *ReliabilityCache
	statsCollector   *statsCollector
	monStats         aggregateStats // TODO(cam): once we verify statsCollector reports data correctly, remove this
	repairOverrides  RepairOverridesMap
	nodeFailureRate  float64
	getNodesEstimate func(ctx context.Context) (int, error)
	log              *zap.Logger

	// we need to delay counting objects to ensure they get associated with the correct redundancy only once
	objectCounted bool
}

func (obs *checkerObserver) getStatsByRS(redundancy storj.RedundancyScheme) *stats {
	rsString := getRSString(obs.loadRedundancy(redundancy))
	return obs.statsCollector.getStatsByRS(rsString)
}

func (obs *checkerObserver) loadRedundancy(redundancy storj.RedundancyScheme) (int, int, int, int) {
	repair := int(redundancy.RepairShares)
	overrideValue := obs.repairOverrides.GetOverrideValue(redundancy)
	if overrideValue != 0 {
		repair = int(overrideValue)
	}
	return int(redundancy.RequiredShares), repair, int(redundancy.OptimalShares), int(redundancy.TotalShares)
}

func (obs *checkerObserver) RemoteSegment(ctx context.Context, segment *metaloop.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	// ignore segment if expired
	if segment.Expired(time.Now()) {
		return nil
	}

	stats := obs.getStatsByRS(segment.Redundancy)

	if !obs.objectCounted {
		obs.objectCounted = true
		stats.iterationAggregates.objectsChecked++
	}

	obs.monStats.remoteSegmentsChecked++
	stats.iterationAggregates.remoteSegmentsChecked++

	// ensure we get values, even if only zero values, so that redash can have an alert based on this
	mon.Counter("checker_segments_below_min_req").Inc(0) //mon:locked
	stats.segmentsBelowMinReq.Inc(0)

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

	totalNumNodes, err := obs.getNodesEstimate(ctx)
	if err != nil {
		return Error.New("could not get estimate of total number of nodes: %w", err)
	}

	createdAt := time.Time{}
	if segment.CreatedAt != nil {
		createdAt = *segment.CreatedAt
	}
	repairedAt := time.Time{}
	if segment.RepairedAt != nil {
		repairedAt = *segment.RepairedAt
	}
	missingPieces, err := obs.nodestate.MissingPieces(ctx, createdAt, segment.Pieces)
	if err != nil {
		obs.monStats.remoteSegmentsFailedToCheck++
		stats.iterationAggregates.remoteSegmentsFailedToCheck++
		return errs.Combine(Error.New("error getting missing pieces"), err)
	}

	numHealthy := len(pieces) - len(missingPieces)
	mon.IntVal("checker_segment_total_count").Observe(int64(len(pieces))) //mon:locked
	stats.segmentTotalCount.Observe(int64(len(pieces)))
	mon.IntVal("checker_segment_healthy_count").Observe(int64(numHealthy)) //mon:locked
	stats.segmentHealthyCount.Observe(int64(numHealthy))

	segmentAge := time.Since(createdAt)
	mon.IntVal("checker_segment_age").Observe(int64(segmentAge.Seconds())) //mon:locked
	stats.segmentAge.Observe(int64(segmentAge.Seconds()))

	required, repairThreshold, successThreshold, _ := obs.loadRedundancy(segment.Redundancy)

	segmentHealth := repair.SegmentHealth(numHealthy, required, totalNumNodes, obs.nodeFailureRate)
	mon.FloatVal("checker_segment_health").Observe(segmentHealth) //mon:locked
	stats.segmentHealth.Observe(segmentHealth)

	key := segment.Location.Encode()
	// we repair when the number of healthy pieces is less than or equal to the repair threshold and is greater or equal to
	// minimum required pieces in redundancy
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	if numHealthy >= required && numHealthy <= repairThreshold && numHealthy < successThreshold {
		mon.FloatVal("checker_injured_segment_health").Observe(segmentHealth) //mon:locked
		stats.injuredSegmentHealth.Observe(segmentHealth)
		obs.monStats.remoteSegmentsNeedingRepair++
		stats.iterationAggregates.remoteSegmentsNeedingRepair++
		alreadyInserted, err := obs.repairQueue.Insert(ctx, &internalpb.InjuredSegment{
			Path:         key,
			LostPieces:   missingPieces,
			InsertedTime: time.Now().UTC(),
		}, segmentHealth)
		if err != nil {
			obs.log.Error("error adding injured segment to queue", zap.Error(err))
			return nil
		}

		if !alreadyInserted {
			obs.monStats.newRemoteSegmentsNeedingRepair++
			stats.iterationAggregates.newRemoteSegmentsNeedingRepair++
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
		if !containsObjectLocation(stats.iterationAggregates.remoteSegmentInfo, lostSegInfo) {
			stats.iterationAggregates.remoteSegmentInfo = append(stats.iterationAggregates.remoteSegmentInfo, lostSegInfo)
		}

		var segmentAge time.Duration
		if createdAt.Before(repairedAt) {
			segmentAge = time.Since(repairedAt)
		} else {
			segmentAge = time.Since(createdAt)
		}
		mon.IntVal("checker_segment_time_until_irreparable").Observe(int64(segmentAge.Seconds())) //mon:locked
		stats.segmentTimeUntilIrreparable.Observe(int64(segmentAge.Seconds()))

		obs.monStats.remoteSegmentsLost++
		stats.iterationAggregates.remoteSegmentsLost++

		mon.Counter("checker_segments_below_min_req").Inc(1) //mon:locked
		stats.segmentsBelowMinReq.Inc(1)

		// make an entry into the irreparable table
		segmentInfo := &internalpb.IrreparableSegment{
			Path:               key,
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
	} else {
		if numHealthy > repairThreshold && numHealthy <= (repairThreshold+len(obs.monStats.remoteSegmentsOverThreshold)) {
			// record metrics for segments right above repair threshold
			// numHealthy=repairThreshold+1 through numHealthy=repairThreshold+5
			for i := range obs.monStats.remoteSegmentsOverThreshold {
				if numHealthy == (repairThreshold + i + 1) {
					obs.monStats.remoteSegmentsOverThreshold[i]++
					break
				}
			}
		}
		if numHealthy > repairThreshold && numHealthy <= (repairThreshold+len(stats.iterationAggregates.remoteSegmentsOverThreshold)) {
			// record metrics for segments right above repair threshold
			// numHealthy=repairThreshold+1 through numHealthy=repairThreshold+5
			for i := range stats.iterationAggregates.remoteSegmentsOverThreshold {
				if numHealthy == (repairThreshold + i + 1) {
					stats.iterationAggregates.remoteSegmentsOverThreshold[i]++
					break
				}
			}
		}
	}

	return nil
}

func (obs *checkerObserver) Object(ctx context.Context, object *metaloop.Object) (err error) {
	defer mon.Task()(&ctx)(&err)

	obs.monStats.objectsChecked++

	// TODO: check for expired objects

	if object.SegmentCount == 0 {
		stats := obs.getStatsByRS(storj.RedundancyScheme{})
		stats.iterationAggregates.objectsChecked++
		return nil
	}
	obs.objectCounted = false

	return nil
}

func (obs *checkerObserver) InlineSegment(ctx context.Context, segment *metaloop.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: check for expired segments

	if !obs.objectCounted {
		// Note: this may give false stats when an object starts with a inline segment.
		obs.objectCounted = true
		stats := obs.getStatsByRS(storj.RedundancyScheme{})
		stats.iterationAggregates.objectsChecked++
	}

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
			var redundancy storj.RedundancyScheme
			var pieces metabase.Pieces
			var createAt time.Time
			if segment.SegmentDetail == (&pb.Pointer{}) {
				// TODO IrreparableDB will be removed in a future so we shouldn't care too much about performance
				location, err := metabase.ParseSegmentKey(metabase.SegmentKey(segment.GetPath()))
				if err != nil {
					return err
				}
				object, err := checker.metabase.GetObjectLatestVersion(ctx, metabase.GetObjectLatestVersion{
					ObjectLocation: location.Object(),
				})
				if err != nil {
					return err
				}

				createAt = object.CreatedAt

				segment, err := checker.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
					StreamID: object.StreamID,
					Position: location.Position,
				})
				if err != nil {
					return err
				}
				redundancy = segment.Redundancy
			} else {
				// skip inline segments
				if segment.SegmentDetail.Remote == nil {
					return nil
				}

				createAt = segment.SegmentDetail.CreationDate

				pbRedundancy := segment.SegmentDetail.Remote.Redundancy
				redundancy = storj.RedundancyScheme{
					RequiredShares: int16(pbRedundancy.MinReq),
					RepairShares:   int16(pbRedundancy.RepairThreshold),
					OptimalShares:  int16(pbRedundancy.SuccessThreshold),
					TotalShares:    int16(pbRedundancy.Total),
					ShareSize:      pbRedundancy.ErasureShareSize,
				}
				pieces = make(metabase.Pieces, len(segment.SegmentDetail.Remote.RemotePieces))
				for _, piece := range segment.SegmentDetail.Remote.RemotePieces {
					pieces = append(pieces, metabase.Piece{
						Number:      uint16(piece.PieceNum),
						StorageNode: piece.NodeId,
					})
				}
			}

			err = checker.updateIrreparableSegmentStatus(ctx,
				metabase.SegmentKey(segment.GetPath()),
				redundancy,
				createAt,
				pieces,
			)
			if err != nil {
				checker.logger.Error("irrepair segment checker failed: ", zap.Error(err))
			}
		}
	}

	return nil
}
