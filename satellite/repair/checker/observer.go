// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/shared/location"
)

var (
	_ rangedloop.Observer = (*Observer)(nil)
	_ rangedloop.Partial  = (*observerFork)(nil)
)

// Observer implements the ranged loop Observer interface.
//
// architecture: Observer
type Observer struct {
	logger                   *zap.Logger
	repairQueue              queue.RepairQueue
	nodesCache               *ReliabilityCache
	repairThresholdOverrides RepairThresholdOverrides
	repairTargetOverrides    RepairTargetOverrides
	nodeFailureRate          float64
	repairQueueBatchSize     int
	excludedCountryCodes     map[location.CountryCode]struct{}
	doDeclumping             bool
	doPlacementCheck         bool
	placements               nodeselection.PlacementDefinitions
	health                   Health

	// the following are reset on each iteration
	startTime  time.Time
	TotalStats aggregateStatsPlacements

	mu             sync.Mutex
	statsCollector map[redundancyStyle]*observerRSStats
}

type redundancyStyle struct {
	Scheme    storj.RedundancyScheme
	Placement storj.PlacementConstraint
}

// NewObserver creates new checker observer instance.
func NewObserver(logger *zap.Logger, repairQueue queue.RepairQueue, overlay *overlay.Service, placements nodeselection.PlacementDefinitions, config Config, health Health) *Observer {
	excludedCountryCodes := make(map[location.CountryCode]struct{})
	for _, countryCode := range config.RepairExcludedCountryCodes {
		if cc := location.ToCountryCode(countryCode); cc != location.None {
			excludedCountryCodes[cc] = struct{}{}
		}
	}

	if config.RepairOverrides.String() != "" {
		// backwards compatibility
		config.RepairThresholdOverrides = RepairThresholdOverrides{config.RepairOverrides}
	}

	nodesCache := NewReliabilityCache(overlay, config.ReliabilityCacheStaleness, config.OnlineWindow)

	return &Observer{
		logger: logger,

		repairQueue:              repairQueue,
		nodesCache:               nodesCache,
		repairThresholdOverrides: config.RepairThresholdOverrides,
		repairTargetOverrides:    config.RepairTargetOverrides,
		nodeFailureRate:          config.NodeFailureRate,
		repairQueueBatchSize:     config.RepairQueueInsertBatchSize,
		excludedCountryCodes:     excludedCountryCodes,
		doDeclumping:             config.DoDeclumping,
		doPlacementCheck:         config.DoPlacementCheck,
		placements:               placements,
		health:                   health,
		statsCollector:           make(map[redundancyStyle]*observerRSStats),
	}
}

// getNodesEstimate updates the estimate of the total number of nodes. It is guaranteed
// to return a number greater than 0 when the error is nil.
//
// We can't calculate this upon first starting a Ranged Loop Observer, because there may not be any
// nodes yet. We expect that there will be nodes before there are segments, though.
func (observer *Observer) getNodesEstimate(ctx context.Context) (int, error) {
	// this should be safe to call frequently; it is an efficient caching lookup.
	totalNumNodes, err := observer.nodesCache.NumNodes(ctx)
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

func (observer *Observer) createInsertBuffer() *queue.InsertBuffer {
	return queue.NewInsertBuffer(observer.repairQueue, observer.repairQueueBatchSize)
}

// TestingCompareInjuredSegmentIDs compares stream id of injured segment.
func (observer *Observer) TestingCompareInjuredSegmentIDs(ctx context.Context, streamIDs []uuid.UUID) error {
	injuredSegments, err := observer.repairQueue.SelectN(ctx, 100)
	if err != nil {
		return err
	}

	var injuredSegmentsIds []uuid.UUID
	for _, segment := range injuredSegments {
		injuredSegmentsIds = append(injuredSegmentsIds, segment.StreamID)
	}

	sort.Slice(injuredSegmentsIds, func(i, j int) bool {
		return injuredSegmentsIds[i].Less(injuredSegmentsIds[j])
	})

	sort.Slice(streamIDs, func(i, j int) bool {
		return streamIDs[i].Less(streamIDs[j])
	})

	if !reflect.DeepEqual(streamIDs, injuredSegmentsIds) {
		return errs.New("injured objects ids are different")
	}

	return nil
}

// Start starts parallel segments loop.
func (observer *Observer) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := observer.nodesCache.Refresh(ctx); err != nil {
		return Error.New("unable to refresh nodes cache: %w", err)
	}

	observer.startTime = startTime
	// Reuse the allocated slice.
	observer.TotalStats = observer.TotalStats[:0]

	return nil
}

// Fork creates a Partial to process a chunk of all the segments.
func (observer *Observer) Fork(ctx context.Context) (_ rangedloop.Partial, err error) {
	defer mon.Task()(&ctx)(&err)

	return newObserverFork(observer), nil
}

// Join is called after the chunk for Partial is done.
// This gives the opportunity to merge the output like in a reduce step.
func (observer *Observer) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer mon.Task()(&ctx)(&err)

	repPartial, ok := partial.(*observerFork)
	if !ok {
		return Error.New("expected partial type %T but got %T", repPartial, partial)
	}

	if err := repPartial.repairQueue.Flush(ctx); err != nil {
		return Error.Wrap(err)
	}

	for rs, partialStats := range repPartial.rsStats {
		observer.statsCollector[rs].iterationAggregates.combine(partialStats.iterationAggregates)
	}

	observer.TotalStats.combine(repPartial.totalStats)

	return nil
}

// Finish is called after all segments are processed by all observers.
func (observer *Observer) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// remove all segments which were not seen as unhealthy by this checker iteration
	healthyDeleted, err := observer.repairQueue.Clean(ctx, observer.startTime)
	if err != nil {
		return Error.Wrap(err)
	}

	observer.collectAggregates()

	var allUnhealthy, allChecked int64
	for p, s := range observer.TotalStats {
		t := monkit.NewSeriesTag("placement", strconv.FormatUint(uint64(p), 10))

		mon.IntVal("remote_files_checked", t).Observe(s.objectsChecked)
		mon.IntVal("remote_segments_checked", t).Observe(s.remoteSegmentsChecked)
		mon.IntVal("remote_segments_failed_to_check", t).Observe(s.remoteSegmentsFailedToCheck)
		mon.IntVal("remote_segments_needing_repair", t).Observe(s.remoteSegmentsNeedingRepair)
		mon.IntVal("remote_segments_needing_repair_due_to_forcing", t).Observe(s.remoteSegmentsNeedingRepairDueToForcing)
		mon.IntVal("new_remote_segments_needing_repair", t).Observe(s.newRemoteSegmentsNeedingRepair)
		mon.IntVal("remote_segments_lost", t).Observe(s.remoteSegmentsLost)
		mon.IntVal("remote_files_lost", t).Observe(int64(len(s.objectsLost)))
		mon.IntVal("remote_segments_over_threshold_1", t).Observe(s.remoteSegmentsOverThreshold[0])
		mon.IntVal("remote_segments_over_threshold_2", t).Observe(s.remoteSegmentsOverThreshold[1])
		mon.IntVal("remote_segments_over_threshold_3", t).Observe(s.remoteSegmentsOverThreshold[2])
		mon.IntVal("remote_segments_over_threshold_4", t).Observe(s.remoteSegmentsOverThreshold[3])
		mon.IntVal("remote_segments_over_threshold_5", t).Observe(s.remoteSegmentsOverThreshold[4])

		allUnhealthy = s.remoteSegmentsNeedingRepair + s.remoteSegmentsFailedToCheck
		allChecked = s.remoteSegmentsChecked
	}

	mon.IntVal("healthy_segments_removed_from_queue").Observe(healthyDeleted)
	allHealthy := allChecked - allUnhealthy
	mon.FloatVal("remote_segments_healthy_percentage").Observe(100 * float64(allHealthy) / float64(allChecked))
	return nil
}

func (observer *Observer) collectAggregates() {
	for _, stats := range observer.statsCollector {
		stats.collectAggregates()
	}
}

func (observer *Observer) getObserverStats(redundancy redundancyStyle) *observerRSStats {
	observer.mu.Lock()
	defer observer.mu.Unlock()

	observerStats, exists := observer.statsCollector[redundancy]
	if !exists {
		adjustedRedundancy := AdjustRedundancy(redundancy.Scheme, observer.repairThresholdOverrides, observer.repairTargetOverrides, observer.placements[redundancy.Placement])
		rsString := fmt.Sprintf("%d/%d/%d/%d", adjustedRedundancy.RequiredShares, adjustedRedundancy.RepairShares, adjustedRedundancy.OptimalShares, adjustedRedundancy.TotalShares)
		observerStats = &observerRSStats{aggregateStats{}, newIterationRSStats(rsString), newSegmentRSStats(rsString, redundancy.Placement)}
		mon.Chain(observerStats)
		observer.statsCollector[redundancy] = observerStats
	}

	return observerStats
}

// RefreshReliabilityCache forces refreshing node online status cache.
func (observer *Observer) RefreshReliabilityCache(ctx context.Context) error {
	return observer.nodesCache.Refresh(ctx)
}

// observerFork implements the ranged loop Partial interface.
type observerFork struct {
	repairQueue              *queue.InsertBuffer
	nodesCache               *ReliabilityCache
	rsStats                  map[redundancyStyle]*partialRSStats
	repairThresholdOverrides RepairThresholdOverrides
	repairTargetOverrides    RepairTargetOverrides
	nodeFailureRate          float64
	getNodesEstimate         func(ctx context.Context) (int, error)
	log                      *zap.Logger
	lastStreamID             uuid.UUID
	totalStats               aggregateStatsPlacements

	// reuse those slices to optimize memory usage
	nodeIDs []storj.NodeID
	nodes   []nodeselection.SelectedNode

	// define from which countries nodes should be marked as offline
	excludedCountryCodes map[location.CountryCode]struct{}
	doDeclumping         bool
	doPlacementCheck     bool
	placements           nodeselection.PlacementDefinitions
	health               Health

	getObserverStats func(redundancyStyle) *observerRSStats
}

// newObserverFork creates new observer partial instance.
func newObserverFork(observer *Observer) rangedloop.Partial {
	// we can only share thread-safe objects.
	return &observerFork{
		repairQueue:              observer.createInsertBuffer(),
		nodesCache:               observer.nodesCache,
		rsStats:                  make(map[redundancyStyle]*partialRSStats),
		repairThresholdOverrides: observer.repairThresholdOverrides,
		repairTargetOverrides:    observer.repairTargetOverrides,
		nodeFailureRate:          observer.nodeFailureRate,
		getNodesEstimate:         observer.getNodesEstimate,
		log:                      observer.logger,
		excludedCountryCodes:     observer.excludedCountryCodes,
		doDeclumping:             observer.doDeclumping,
		doPlacementCheck:         observer.doPlacementCheck,
		placements:               observer.placements,
		health:                   observer.health,
		getObserverStats:         observer.getObserverStats,
	}
}

func (fork *observerFork) getStatsByRS(redundancy redundancyStyle) *partialRSStats {
	stats, ok := fork.rsStats[redundancy]
	if !ok {
		observerStats := fork.getObserverStats(redundancy)

		fork.rsStats[redundancy] = &partialRSStats{
			iterationAggregates: aggregateStats{},
			segmentStats:        observerStats.segmentStats,
		}
		return fork.rsStats[redundancy]
	}

	return stats
}

// Process is called repeatedly with batches of segments. It is not called
// concurrently on the same instance. Method is not concurrent-safe on it own.
func (fork *observerFork) Process(ctx context.Context, segments []rangedloop.Segment) (err error) {
	for _, segment := range segments {
		if err := fork.process(ctx, &segment); err != nil {
			return err
		}
	}

	return nil
}

var (
	// initialize monkit metrics once for better performance.
	segmentTotalCountIntVal           = mon.IntVal("checker_segment_total_count")
	segmentClumpedCountIntVal         = mon.IntVal("checker_segment_clumped_count")
	segmentExitingCountIntVal         = mon.IntVal("checker_segment_exiting_count")
	segmentAgeIntVal                  = mon.IntVal("checker_segment_age")
	segmentFreshnessIntVal            = mon.IntVal("checker_segment_freshness")
	segmentHealthFloatVal             = mon.FloatVal("checker_segment_health")
	segmentsBelowMinReqCounter        = mon.Counter("checker_segments_below_min_req")
	injuredSegmentHealthFloatVal      = mon.FloatVal("checker_injured_segment_health")
	segmentTimeUntilIrreparableIntVal = mon.IntVal("checker_segment_time_until_irreparable")

	allSegmentPiecesLostPerWeekFloatVal        = mon.FloatVal("checker_all_segment_pieces_lost_per_week")
	freshSegmentPiecesLostPerWeekFloatVal      = mon.FloatVal("checker_fresh_segment_pieces_lost_per_week")
	weekOldSegmentPiecesLostPerWeekFloatVal    = mon.FloatVal("checker_week_old_segment_pieces_lost_per_week")
	monthOldSegmentPiecesLostPerWeekFloatVal   = mon.FloatVal("checker_month_old_segment_pieces_lost_per_week")
	quarterOldSegmentPiecesLostPerWeekFloatVal = mon.FloatVal("checker_quarter_old_segment_pieces_lost_per_week")
	yearOldSegmentPiecesLostPerWeekFloatVal    = mon.FloatVal("checker_year_old_segment_pieces_lost_per_week")
)

func (fork *observerFork) process(ctx context.Context, segment *rangedloop.Segment) (err error) {
	// Grow the fork.totalStats if this placement doesn't fit.
	if l := int(segment.Placement+1) - len(fork.totalStats); l > 0 {
		fork.totalStats = append(fork.totalStats, make([]aggregateStats, l)...)
	}

	if segment.Inline() {
		if fork.lastStreamID.Compare(segment.StreamID) != 0 {
			fork.lastStreamID = segment.StreamID
			fork.totalStats[segment.Placement].objectsChecked++
		}

		return nil
	}

	// ignore segment if expired
	if segment.Expired(time.Now()) {
		return nil
	}

	stats := fork.getStatsByRS(redundancyStyle{
		Scheme:    segment.Redundancy,
		Placement: segment.Placement,
	})
	if fork.lastStreamID.Compare(segment.StreamID) != 0 {
		fork.lastStreamID = segment.StreamID
		stats.iterationAggregates.objectsChecked++
		fork.totalStats[segment.Placement].objectsChecked++
	}

	fork.totalStats[segment.Placement].remoteSegmentsChecked++
	stats.iterationAggregates.remoteSegmentsChecked++

	log := fork.log.With(zap.Object("Segment", segment))

	// ensure we get values, even if only zero values, so that redash can have an alert based on this
	segmentsBelowMinReqCounter.Inc(0)
	pieces := segment.Pieces
	if len(pieces) == 0 {
		log.Debug("no pieces on remote segment")
		return nil
	}

	// reuse fork.nodeIDs and fork.nodes slices if large enough
	if cap(fork.nodeIDs) < len(pieces) {
		fork.nodeIDs = make([]storj.NodeID, len(pieces))
		fork.nodes = make([]nodeselection.SelectedNode, len(pieces))
	} else {
		fork.nodeIDs = fork.nodeIDs[:len(pieces)]
		fork.nodes = fork.nodes[:len(pieces)]
	}

	for i, piece := range pieces {
		fork.nodeIDs[i] = piece.StorageNode
	}
	selectedNodes, err := fork.nodesCache.GetNodes(ctx, segment.CreatedAt, fork.nodeIDs, fork.nodes)
	if err != nil {
		fork.totalStats[segment.Placement].remoteSegmentsFailedToCheck++
		stats.iterationAggregates.remoteSegmentsFailedToCheck++
		return Error.New("error getting node information for pieces: %w", err)
	}
	piecesCheck := repair.ClassifySegmentPieces(segment.Pieces, selectedNodes, fork.excludedCountryCodes, fork.doPlacementCheck,
		fork.doDeclumping, fork.placements[segment.Placement])

	segmentTotalCountIntVal.Observe(int64(len(pieces)))
	stats.segmentStats.segmentTotalCount.Observe(int64(len(pieces)))

	numHealthy := piecesCheck.Healthy.Count()
	mon.IntVal("checker_segment_healthy_count", monkit.NewSeriesTag(
		"placement", strconv.FormatUint(uint64(segment.Placement), 10),
	)).Observe(int64(numHealthy))

	stats.segmentStats.segmentHealthyCount.Observe(int64(numHealthy))

	segmentClumpedCountIntVal.Observe(int64(piecesCheck.Clumped.Count()))
	stats.segmentStats.segmentClumpedCount.Observe(int64(piecesCheck.Clumped.Count()))
	segmentExitingCountIntVal.Observe(int64(piecesCheck.Exiting.Count()))
	stats.segmentStats.segmentExitingCount.Observe(int64(piecesCheck.Exiting.Count()))
	mon.IntVal("checker_segment_off_placement_count",
		monkit.NewSeriesTag("placement", strconv.Itoa(int(segment.Placement)))).Observe(int64(piecesCheck.OutOfPlacement.Count()))
	stats.segmentStats.segmentOffPlacementCount.Observe(int64(piecesCheck.OutOfPlacement.Count()))

	segmentAge := time.Since(segment.CreatedAt)
	segmentAgeIntVal.Observe(int64(segmentAge.Seconds()))
	stats.segmentStats.segmentAge.Observe(int64(segmentAge.Seconds()))

	segmentFreshness := segmentAge
	if segment.RepairedAt != nil && segment.RepairedAt.After(segment.CreatedAt) {
		segmentFreshness = time.Since(*segment.RepairedAt)
	}
	segmentFreshnessIntVal.Observe(int64(segmentFreshness.Seconds()))
	stats.segmentStats.segmentFreshness.Observe(int64(segmentFreshness.Seconds()))

	lostPieces := len(pieces) - numHealthy
	const weekSeconds = 60 * 60 * 24 * 7
	piecesLostPerWeek := float64(lostPieces) / (segmentFreshness.Seconds() / weekSeconds)

	allSegmentPiecesLostPerWeekFloatVal.Observe(piecesLostPerWeek)
	stats.segmentStats.allSegmentPiecesLostPerWeek.Observe(piecesLostPerWeek)
	if segmentFreshness.Seconds() < weekSeconds {
		freshSegmentPiecesLostPerWeekFloatVal.Observe(piecesLostPerWeek)
		stats.segmentStats.freshSegmentPiecesLostPerWeek.Observe(piecesLostPerWeek)
	} else if segmentFreshness.Seconds() < 4*weekSeconds {
		weekOldSegmentPiecesLostPerWeekFloatVal.Observe(piecesLostPerWeek)
		stats.segmentStats.weekOldSegmentPiecesLostPerWeek.Observe(piecesLostPerWeek)
	} else if segmentFreshness.Seconds() < 3*4*weekSeconds {
		monthOldSegmentPiecesLostPerWeekFloatVal.Observe(piecesLostPerWeek)
		stats.segmentStats.monthOldSegmentPiecesLostPerWeek.Observe(piecesLostPerWeek)
	} else if segmentFreshness.Seconds() < 52*weekSeconds {
		quarterOldSegmentPiecesLostPerWeekFloatVal.Observe(piecesLostPerWeek)
		stats.segmentStats.quarterOldSegmentPiecesLostPerWeek.Observe(piecesLostPerWeek)
	} else {
		yearOldSegmentPiecesLostPerWeekFloatVal.Observe(piecesLostPerWeek)
		stats.segmentStats.yearOldSegmentPiecesLostPerWeek.Observe(piecesLostPerWeek)
	}

	adjustedRedundancy := AdjustRedundancy(segment.Redundancy, fork.repairThresholdOverrides, fork.repairTargetOverrides, fork.placements[segment.Placement])
	segmentHealth := fork.health.Calculate(ctx, numHealthy, int(adjustedRedundancy.RequiredShares), piecesCheck.ForcingRepair.Count())
	segmentHealthFloatVal.Observe(segmentHealth)
	stats.segmentStats.segmentHealth.Observe(segmentHealth)

	// we repair when the number of healthy pieces is less than or equal to the repair threshold and is greater or equal to
	// minimum required pieces in redundancy
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing).
	// separate case is when we find pieces which are outside segment placement. in such case we are putting segment
	// into queue right away.
	repairDueToHealth := numHealthy <= int(adjustedRedundancy.RepairShares) && numHealthy < int(adjustedRedundancy.OptimalShares)
	repairDueToForcing := piecesCheck.ForcingRepair.Count() > 0
	if repairDueToHealth || repairDueToForcing {

		injuredSegmentHealthFloatVal.Observe(segmentHealth)
		stats.segmentStats.injuredSegmentHealth.Observe(segmentHealth)
		fork.totalStats[segment.Placement].remoteSegmentsNeedingRepair++
		stats.iterationAggregates.remoteSegmentsNeedingRepair++

		if repairDueToForcing && !repairDueToHealth {
			fork.totalStats[segment.Placement].remoteSegmentsNeedingRepairDueToForcing++
			stats.iterationAggregates.remoteSegmentsNeedingRepairDueToForcing++
		}

		err := fork.repairQueue.Insert(ctx, &queue.InjuredSegment{
			StreamID:                 segment.StreamID,
			Position:                 segment.Position,
			UpdatedAt:                time.Now().UTC(),
			SegmentHealth:            segmentHealth,
			Placement:                segment.Placement,
			NumNormalizedHealthy:     int16(piecesCheck.Healthy.Count()) - segment.Redundancy.RequiredShares,
			NumNormalizedRetrievable: int16(piecesCheck.Retrievable.Count()) - segment.Redundancy.RequiredShares,
			NumOutOfPlacement:        int16(piecesCheck.OutOfPlacement.Count()),
		}, func() {
			// Counters are increased after the queue has determined
			// that the segment wasn't already queued for repair.
			fork.totalStats[segment.Placement].newRemoteSegmentsNeedingRepair++
			stats.iterationAggregates.newRemoteSegmentsNeedingRepair++
		})
		if err != nil {
			log.Error("error adding injured segment to queue", zap.Error(err))
			return nil
		}

		log := log.With(zap.Int16("Repair Threshold", adjustedRedundancy.RepairShares), zap.Int16("Success Threshold", adjustedRedundancy.OptimalShares),
			zap.Int("Total Pieces", len(pieces)), zap.Int16("Min Required", adjustedRedundancy.RequiredShares))

		switch {
		case piecesCheck.Retrievable.Count() < int(adjustedRedundancy.RequiredShares):
			// monitor irreparable segments
			if !slices.Contains(fork.totalStats[segment.Placement].objectsLost, segment.StreamID) {
				fork.totalStats[segment.Placement].objectsLost = append(
					fork.totalStats[segment.Placement].objectsLost, segment.StreamID,
				)
			}

			if !slices.Contains(stats.iterationAggregates.objectsLost, segment.StreamID) {
				stats.iterationAggregates.objectsLost = append(stats.iterationAggregates.objectsLost, segment.StreamID)
			}

			segmentTimeUntilIrreparableIntVal.Observe(int64(segmentFreshness.Seconds()))
			stats.segmentStats.segmentTimeUntilIrreparable.Observe(int64(segmentFreshness.Seconds()))

			fork.totalStats[segment.Placement].remoteSegmentsLost++
			stats.iterationAggregates.remoteSegmentsLost++

			segmentsBelowMinReqCounter.Inc(1)
			stats.segmentStats.segmentsBelowMinReq.Inc(1)

			var missingNodes []string
			for _, piece := range pieces {
				if piecesCheck.Missing.Contains(int(piece.Number)) {
					missingNodes = append(missingNodes, piece.StorageNode.String())
				}
			}
			log.Warn("checker found irreparable segment",
				zap.String("Unavailable Node IDs", strings.Join(missingNodes, ",")))

		case piecesCheck.Clumped.Count() > 0 && piecesCheck.Healthy.Count()+piecesCheck.Clumped.Count() > int(adjustedRedundancy.RepairShares) &&
			piecesCheck.ForcingRepair.Count() == 0:

			// This segment is to be repaired because of clumping (it wouldn't need repair yet
			// otherwise). Produce a brief report of where the clumping occurred so that we have
			// a better understanding of the cause.
			lastNets := make([]string, len(pieces))
			for i, node := range selectedNodes {
				lastNets[i] = node.LastNet
			}
			clumpedNets := clumpingReport{lastNets: lastNets}
			log.Debug("segment needs repair only because of clumping",
				zap.Stringer("Clumping", &clumpedNets))
		default:
			log.Debug("segment requires repair", zap.Object("Classification", piecesCheck))
		}

		return nil
	}

	if numHealthy > int(adjustedRedundancy.RepairShares) && numHealthy <= (int(adjustedRedundancy.RepairShares)+len(
		fork.totalStats[segment.Placement].remoteSegmentsOverThreshold,
	)) {
		// record metrics for segments right above repair threshold
		// numHealthy=repairThreshold+1 through numHealthy=repairThreshold+5
		for i := range fork.totalStats[segment.Placement].remoteSegmentsOverThreshold {
			if numHealthy == (int(adjustedRedundancy.RepairShares) + i + 1) {
				fork.totalStats[segment.Placement].remoteSegmentsOverThreshold[i]++
				break
			}
		}
	}

	if numHealthy > int(adjustedRedundancy.RepairShares) && numHealthy <= (int(adjustedRedundancy.RepairShares)+len(stats.iterationAggregates.remoteSegmentsOverThreshold)) {
		// record metrics for segments right above repair threshold
		// numHealthy=repairThreshold+1 through numHealthy=repairThreshold+5
		for i := range stats.iterationAggregates.remoteSegmentsOverThreshold {
			if numHealthy == (int(adjustedRedundancy.RepairShares) + i + 1) {
				stats.iterationAggregates.remoteSegmentsOverThreshold[i]++
				break
			}
		}
	}

	return nil
}

type clumpingReport struct {
	lastNets []string
}

// String produces the clumping report. In case the satellite isn't logging at the required level,
// we avoid doing the work of building the report until String() is called.
func (cr *clumpingReport) String() string {
	netCounts := make(map[string]int)
	for _, lastNet := range cr.lastNets {
		if lastNet == "" {
			continue
		}
		netCounts[lastNet]++
	}
	counts := make([]string, 0, len(netCounts))
	for lastNet, count := range netCounts {
		if count > 1 {
			counts = append(counts, fmt.Sprintf("[%s]: %d", lastNet, count))
		}
	}
	return strings.Join(counts, ", ")
}
