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
	"storj.io/common/storj/location"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair"
	"storj.io/storj/satellite/repair/queue"
)

var (
	_ rangedloop.Observer = (*Observer)(nil)
	_ rangedloop.Partial  = (*observerFork)(nil)
)

// Observer implements the ranged loop Observer interface.
//
// architecture: Observer
type Observer struct {
	logger               *zap.Logger
	repairQueue          queue.RepairQueue
	nodesCache           *ReliabilityCache
	overlayService       *overlay.Service
	repairOverrides      RepairOverridesMap
	nodeFailureRate      float64
	repairQueueBatchSize int
	excludedCountryCodes map[location.CountryCode]struct{}
	doDeclumping         bool
	doPlacementCheck     bool
	placements           nodeselection.PlacementDefinitions

	// the following are reset on each iteration
	startTime  time.Time
	TotalStats aggregateStatsPlacements

	mu             sync.Mutex
	statsCollector map[storj.RedundancyScheme]*observerRSStats
}

// NewObserver creates new checker observer instance.
func NewObserver(logger *zap.Logger, repairQueue queue.RepairQueue, overlay *overlay.Service, placements nodeselection.PlacementDefinitions, config Config) *Observer {
	excludedCountryCodes := make(map[location.CountryCode]struct{})
	for _, countryCode := range config.RepairExcludedCountryCodes {
		if cc := location.ToCountryCode(countryCode); cc != location.None {
			excludedCountryCodes[cc] = struct{}{}
		}
	}

	return &Observer{
		logger: logger,

		repairQueue:          repairQueue,
		nodesCache:           NewReliabilityCache(overlay, config.ReliabilityCacheStaleness),
		overlayService:       overlay,
		repairOverrides:      config.RepairOverrides.GetMap(),
		nodeFailureRate:      config.NodeFailureRate,
		repairQueueBatchSize: config.RepairQueueInsertBatchSize,
		excludedCountryCodes: excludedCountryCodes,
		doDeclumping:         config.DoDeclumping,
		doPlacementCheck:     config.DoPlacementCheck,
		placements:           placements,
		statsCollector:       make(map[storj.RedundancyScheme]*observerRSStats),
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

		mon.IntVal("remote_files_checked", t).Observe(s.objectsChecked)                               //mon:locked
		mon.IntVal("remote_segments_checked", t).Observe(s.remoteSegmentsChecked)                     //mon:locked
		mon.IntVal("remote_segments_failed_to_check", t).Observe(s.remoteSegmentsFailedToCheck)       //mon:locked
		mon.IntVal("remote_segments_needing_repair", t).Observe(s.remoteSegmentsNeedingRepair)        //mon:locked
		mon.IntVal("new_remote_segments_needing_repair", t).Observe(s.newRemoteSegmentsNeedingRepair) //mon:locked
		mon.IntVal("remote_segments_lost", t).Observe(s.remoteSegmentsLost)                           //mon:locked
		mon.IntVal("remote_files_lost", t).Observe(int64(len(s.objectsLost)))                         //mon:locked
		mon.IntVal("remote_segments_over_threshold_1", t).Observe(s.remoteSegmentsOverThreshold[0])   //mon:locked
		mon.IntVal("remote_segments_over_threshold_2", t).Observe(s.remoteSegmentsOverThreshold[1])   //mon:locked
		mon.IntVal("remote_segments_over_threshold_3", t).Observe(s.remoteSegmentsOverThreshold[2])   //mon:locked
		mon.IntVal("remote_segments_over_threshold_4", t).Observe(s.remoteSegmentsOverThreshold[3])   //mon:locked
		mon.IntVal("remote_segments_over_threshold_5", t).Observe(s.remoteSegmentsOverThreshold[4])   //mon:locked

		allUnhealthy = s.remoteSegmentsNeedingRepair + s.remoteSegmentsFailedToCheck
		allChecked = s.remoteSegmentsChecked
	}

	mon.IntVal("healthy_segments_removed_from_queue").Observe(healthyDeleted) //mon:locked
	allHealthy := allChecked - allUnhealthy
	mon.FloatVal("remote_segments_healthy_percentage").Observe(100 * float64(allHealthy) / float64(allChecked)) //mon:locked
	return nil
}

func (observer *Observer) collectAggregates() {
	for _, stats := range observer.statsCollector {
		stats.collectAggregates()
	}
}

func (observer *Observer) getObserverStats(redundancy storj.RedundancyScheme) *observerRSStats {
	observer.mu.Lock()
	defer observer.mu.Unlock()

	observerStats, exists := observer.statsCollector[redundancy]
	if !exists {
		rsString := getRSString(loadRedundancy(redundancy, observer.repairOverrides))
		observerStats = &observerRSStats{aggregateStats{}, newIterationRSStats(rsString), newSegmentRSStats(rsString)}
		mon.Chain(observerStats)
		observer.statsCollector[redundancy] = observerStats
	}

	return observerStats
}

func loadRedundancy(redundancy storj.RedundancyScheme, repairOverrides RepairOverridesMap) (int, int, int, int) {
	repair := int(redundancy.RepairShares)

	overrideValue := repairOverrides.GetOverrideValue(redundancy)
	if overrideValue != 0 {
		repair = int(overrideValue)
	}

	return int(redundancy.RequiredShares), repair, int(redundancy.OptimalShares), int(redundancy.TotalShares)
}

// RefreshReliabilityCache forces refreshing node online status cache.
func (observer *Observer) RefreshReliabilityCache(ctx context.Context) error {
	return observer.nodesCache.Refresh(ctx)
}

// observerFork implements the ranged loop Partial interface.
type observerFork struct {
	repairQueue      *queue.InsertBuffer
	nodesCache       *ReliabilityCache
	overlayService   *overlay.Service
	rsStats          map[storj.RedundancyScheme]*partialRSStats
	repairOverrides  RepairOverridesMap
	nodeFailureRate  float64
	getNodesEstimate func(ctx context.Context) (int, error)
	log              *zap.Logger
	lastStreamID     uuid.UUID
	totalStats       aggregateStatsPlacements

	// reuse those slices to optimize memory usage
	nodeIDs []storj.NodeID
	nodes   []nodeselection.SelectedNode

	// define from which countries nodes should be marked as offline
	excludedCountryCodes map[location.CountryCode]struct{}
	doDeclumping         bool
	doPlacementCheck     bool
	placements           nodeselection.PlacementDefinitions

	getObserverStats func(storj.RedundancyScheme) *observerRSStats
}

// newObserverFork creates new observer partial instance.
func newObserverFork(observer *Observer) rangedloop.Partial {
	// we can only share thread-safe objects.
	return &observerFork{
		repairQueue:          observer.createInsertBuffer(),
		nodesCache:           observer.nodesCache,
		overlayService:       observer.overlayService,
		rsStats:              make(map[storj.RedundancyScheme]*partialRSStats),
		repairOverrides:      observer.repairOverrides,
		nodeFailureRate:      observer.nodeFailureRate,
		getNodesEstimate:     observer.getNodesEstimate,
		log:                  observer.logger,
		excludedCountryCodes: observer.excludedCountryCodes,
		doDeclumping:         observer.doDeclumping,
		doPlacementCheck:     observer.doPlacementCheck,
		placements:           observer.placements,
		getObserverStats:     observer.getObserverStats,
	}
}

func (fork *observerFork) getStatsByRS(redundancy storj.RedundancyScheme) *partialRSStats {
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
	segmentTotalCountIntVal           = mon.IntVal("checker_segment_total_count")   //mon:locked
	segmentClumpedCountIntVal         = mon.IntVal("checker_segment_clumped_count") //mon:locked
	segmentExitingCountIntVal         = mon.IntVal("checker_segment_exiting_count")
	segmentAgeIntVal                  = mon.IntVal("checker_segment_age")                    //mon:locked
	segmentHealthFloatVal             = mon.FloatVal("checker_segment_health")               //mon:locked
	segmentsBelowMinReqCounter        = mon.Counter("checker_segments_below_min_req")        //mon:locked
	injuredSegmentHealthFloatVal      = mon.FloatVal("checker_injured_segment_health")       //mon:locked
	segmentTimeUntilIrreparableIntVal = mon.IntVal("checker_segment_time_until_irreparable") //mon:locked
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

	stats := fork.getStatsByRS(segment.Redundancy)
	if fork.lastStreamID.Compare(segment.StreamID) != 0 {
		fork.lastStreamID = segment.StreamID
		stats.iterationAggregates.objectsChecked++
		fork.totalStats[segment.Placement].objectsChecked++
	}

	fork.totalStats[segment.Placement].remoteSegmentsChecked++
	stats.iterationAggregates.remoteSegmentsChecked++

	// ensure we get values, even if only zero values, so that redash can have an alert based on this
	segmentsBelowMinReqCounter.Inc(0)
	pieces := segment.Pieces
	if len(pieces) == 0 {
		fork.log.Debug("no pieces on remote segment")
		return nil
	}

	totalNumNodes, err := fork.getNodesEstimate(ctx)
	if err != nil {
		return Error.New("could not get estimate of total number of nodes: %w", err)
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

	numHealthy := piecesCheck.Healthy.Count()
	segmentTotalCountIntVal.Observe(int64(len(pieces)))
	stats.segmentStats.segmentTotalCount.Observe(int64(len(pieces)))

	mon.IntVal("checker_segment_healthy_count", monkit.NewSeriesTag(
		"placement", strconv.FormatUint(uint64(segment.Placement), 10),
	)).Observe(int64(numHealthy)) //mon:locked

	stats.segmentStats.segmentHealthyCount.Observe(int64(numHealthy))

	segmentClumpedCountIntVal.Observe(int64(piecesCheck.Clumped.Count()))
	stats.segmentStats.segmentClumpedCount.Observe(int64(piecesCheck.Clumped.Count()))
	segmentExitingCountIntVal.Observe(int64(piecesCheck.Exiting.Count()))
	stats.segmentStats.segmentExitingCount.Observe(int64(piecesCheck.Exiting.Count()))
	mon.IntVal("checker_segment_off_placement_count",
		monkit.NewSeriesTag("placement", strconv.Itoa(int(segment.Placement)))).Observe(int64(piecesCheck.OutOfPlacement.Count())) //mon:locked
	stats.segmentStats.segmentOffPlacementCount.Observe(int64(piecesCheck.OutOfPlacement.Count()))

	segmentAge := time.Since(segment.CreatedAt)
	segmentAgeIntVal.Observe(int64(segmentAge.Seconds()))
	stats.segmentStats.segmentAge.Observe(int64(segmentAge.Seconds()))

	required, repairThreshold, successThreshold, _ := loadRedundancy(segment.Redundancy, fork.repairOverrides)
	segmentHealth := repair.SegmentHealth(numHealthy, required, totalNumNodes, fork.nodeFailureRate, piecesCheck.ForcingRepair.Count())
	segmentHealthFloatVal.Observe(segmentHealth)
	stats.segmentStats.segmentHealth.Observe(segmentHealth)

	// we repair when the number of healthy pieces is less than or equal to the repair threshold and is greater or equal to
	// minimum required pieces in redundancy
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing).
	// separate case is when we find pieces which are outside segment placement. in such case we are putting segment
	// into queue right away.
	if (numHealthy <= repairThreshold && numHealthy < successThreshold) || piecesCheck.ForcingRepair.Count() > 0 {
		injuredSegmentHealthFloatVal.Observe(segmentHealth)
		stats.segmentStats.injuredSegmentHealth.Observe(segmentHealth)
		fork.totalStats[segment.Placement].remoteSegmentsNeedingRepair++
		stats.iterationAggregates.remoteSegmentsNeedingRepair++
		err := fork.repairQueue.Insert(ctx, &queue.InjuredSegment{
			StreamID:      segment.StreamID,
			Position:      segment.Position,
			UpdatedAt:     time.Now().UTC(),
			SegmentHealth: segmentHealth,
			Placement:     segment.Placement,
		}, func() {
			// Counters are increased after the queue has determined
			// that the segment wasn't already queued for repair.
			fork.totalStats[segment.Placement].newRemoteSegmentsNeedingRepair++
			stats.iterationAggregates.newRemoteSegmentsNeedingRepair++
		})
		if err != nil {
			fork.log.Error("error adding injured segment to queue", zap.Error(err))
			return nil
		}

		// monitor irreparable segments
		if piecesCheck.Retrievable.Count() < required {
			if !slices.Contains(fork.totalStats[segment.Placement].objectsLost, segment.StreamID) {
				fork.totalStats[segment.Placement].objectsLost = append(
					fork.totalStats[segment.Placement].objectsLost, segment.StreamID,
				)
			}

			if !slices.Contains(stats.iterationAggregates.objectsLost, segment.StreamID) {
				stats.iterationAggregates.objectsLost = append(stats.iterationAggregates.objectsLost, segment.StreamID)
			}

			repairedAt := time.Time{}
			if segment.RepairedAt != nil {
				repairedAt = *segment.RepairedAt
			}

			var segmentAge time.Duration
			if segment.CreatedAt.Before(repairedAt) {
				segmentAge = time.Since(repairedAt)
			} else {
				segmentAge = time.Since(segment.CreatedAt)
			}

			segmentTimeUntilIrreparableIntVal.Observe(int64(segmentAge.Seconds()))
			stats.segmentStats.segmentTimeUntilIrreparable.Observe(int64(segmentAge.Seconds()))

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
			fork.log.Warn("checker found irreparable segment", zap.String("Segment StreamID", segment.StreamID.String()), zap.Int("Segment Position",
				int(segment.Position.Encode())), zap.Int("total pieces", len(pieces)), zap.Int("min required", required), zap.String("unavailable node IDs", strings.Join(missingNodes, ",")))
		} else if piecesCheck.Clumped.Count() > 0 && piecesCheck.Healthy.Count()+piecesCheck.Clumped.Count() > repairThreshold && piecesCheck.ForcingRepair.Count() == 0 {
			// This segment is to be repaired because of clumping (it wouldn't need repair yet
			// otherwise). Produce a brief report of where the clumping occurred so that we have
			// a better understanding of the cause.
			lastNets := make([]string, len(pieces))
			for i, node := range selectedNodes {
				lastNets[i] = node.LastNet
			}
			clumpedNets := clumpingReport{lastNets: lastNets}
			fork.log.Info("segment needs repair only because of clumping", zap.Stringer("Segment StreamID", segment.StreamID), zap.Uint64("Segment Position", segment.Position.Encode()), zap.Int("total pieces", len(pieces)), zap.Int("min required", required), zap.Stringer("clumping", &clumpedNets))
		}
	} else {
		if numHealthy > repairThreshold && numHealthy <= (repairThreshold+len(
			fork.totalStats[segment.Placement].remoteSegmentsOverThreshold,
		)) {
			// record metrics for segments right above repair threshold
			// numHealthy=repairThreshold+1 through numHealthy=repairThreshold+5
			for i := range fork.totalStats[segment.Placement].remoteSegmentsOverThreshold {
				if numHealthy == (repairThreshold + i + 1) {
					fork.totalStats[segment.Placement].remoteSegmentsOverThreshold[i]++
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
