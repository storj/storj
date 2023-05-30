// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair"
	"storj.io/storj/satellite/repair/queue"
)

var _ rangedloop.Observer = (*Observer)(nil)
var _ rangedloop.Partial = (*observerFork)(nil)

// Observer implements the ranged loop Observer interface.
//
// architecture: Observer
type Observer struct {
	logger               *zap.Logger
	repairQueue          queue.RepairQueue
	nodestate            *ReliabilityCache
	overlayService       *overlay.Service
	repairOverrides      RepairOverridesMap
	nodeFailureRate      float64
	repairQueueBatchSize int
	doDeclumping         bool
	doPlacementCheck     bool

	// the following are reset on each iteration
	startTime  time.Time
	TotalStats aggregateStats

	mu             sync.Mutex
	statsCollector map[string]*observerRSStats
}

// NewObserver creates new checker observer instance.
func NewObserver(logger *zap.Logger, repairQueue queue.RepairQueue, overlay *overlay.Service, config Config) *Observer {
	return &Observer{
		logger: logger,

		repairQueue:          repairQueue,
		nodestate:            NewReliabilityCache(overlay, config.ReliabilityCacheStaleness),
		overlayService:       overlay,
		repairOverrides:      config.RepairOverrides.GetMap(),
		nodeFailureRate:      config.NodeFailureRate,
		repairQueueBatchSize: config.RepairQueueInsertBatchSize,
		doDeclumping:         config.DoDeclumping,
		doPlacementCheck:     config.DoPlacementCheck,
		statsCollector:       make(map[string]*observerRSStats),
	}
}

// getNodesEstimate updates the estimate of the total number of nodes. It is guaranteed
// to return a number greater than 0 when the error is nil.
//
// We can't calculate this upon first starting a Ranged Loop Observer, because there may not be any
// nodes yet. We expect that there will be nodes before there are segments, though.
func (observer *Observer) getNodesEstimate(ctx context.Context) (int, error) {
	// this should be safe to call frequently; it is an efficient caching lookup.
	totalNumNodes, err := observer.nodestate.NumNodes(ctx)
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
	observer.TotalStats = aggregateStats{}

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

	mon.IntVal("remote_files_checked").Observe(observer.TotalStats.objectsChecked)                               //mon:locked
	mon.IntVal("remote_segments_checked").Observe(observer.TotalStats.remoteSegmentsChecked)                     //mon:locked
	mon.IntVal("remote_segments_failed_to_check").Observe(observer.TotalStats.remoteSegmentsFailedToCheck)       //mon:locked
	mon.IntVal("remote_segments_needing_repair").Observe(observer.TotalStats.remoteSegmentsNeedingRepair)        //mon:locked
	mon.IntVal("new_remote_segments_needing_repair").Observe(observer.TotalStats.newRemoteSegmentsNeedingRepair) //mon:locked
	mon.IntVal("remote_segments_lost").Observe(observer.TotalStats.remoteSegmentsLost)                           //mon:locked
	mon.IntVal("remote_files_lost").Observe(int64(len(observer.TotalStats.objectsLost)))                         //mon:locked
	mon.IntVal("remote_segments_over_threshold_1").Observe(observer.TotalStats.remoteSegmentsOverThreshold[0])   //mon:locked
	mon.IntVal("remote_segments_over_threshold_2").Observe(observer.TotalStats.remoteSegmentsOverThreshold[1])   //mon:locked
	mon.IntVal("remote_segments_over_threshold_3").Observe(observer.TotalStats.remoteSegmentsOverThreshold[2])   //mon:locked
	mon.IntVal("remote_segments_over_threshold_4").Observe(observer.TotalStats.remoteSegmentsOverThreshold[3])   //mon:locked
	mon.IntVal("remote_segments_over_threshold_5").Observe(observer.TotalStats.remoteSegmentsOverThreshold[4])   //mon:locked
	mon.IntVal("healthy_segments_removed_from_queue").Observe(healthyDeleted)                                    //mon:locked
	allUnhealthy := observer.TotalStats.remoteSegmentsNeedingRepair + observer.TotalStats.remoteSegmentsFailedToCheck
	allChecked := observer.TotalStats.remoteSegmentsChecked
	allHealthy := allChecked - allUnhealthy
	mon.FloatVal("remote_segments_healthy_percentage").Observe(100 * float64(allHealthy) / float64(allChecked)) //mon:locked

	return nil
}

func (observer *Observer) collectAggregates() {
	for _, stats := range observer.statsCollector {
		stats.collectAggregates()
	}
}

func (observer *Observer) getObserverStats(rsString string) *observerRSStats {
	observer.mu.Lock()
	defer observer.mu.Unlock()

	observerStats, exists := observer.statsCollector[rsString]
	if !exists {
		observerStats = &observerRSStats{aggregateStats{}, newIterationRSStats(rsString), newSegmentRSStats(rsString)}
		mon.Chain(observerStats)
		observer.statsCollector[rsString] = observerStats
	}

	return observerStats
}

// RefreshReliabilityCache forces refreshing node online status cache.
func (observer *Observer) RefreshReliabilityCache(ctx context.Context) error {
	return observer.nodestate.Refresh(ctx)
}

// observerFork implements the ranged loop Partial interface.
type observerFork struct {
	repairQueue      *queue.InsertBuffer
	nodestate        *ReliabilityCache
	overlayService   *overlay.Service
	rsStats          map[string]*partialRSStats
	repairOverrides  RepairOverridesMap
	nodeFailureRate  float64
	getNodesEstimate func(ctx context.Context) (int, error)
	log              *zap.Logger
	doDeclumping     bool
	doPlacementCheck bool
	lastStreamID     uuid.UUID
	totalStats       aggregateStats
	allNodeIDs       []storj.NodeID

	getObserverStats func(string) *observerRSStats
}

// newObserverFork creates new observer partial instance.
func newObserverFork(observer *Observer) rangedloop.Partial {
	// we can only share thread-safe objects.
	return &observerFork{
		repairQueue:      observer.createInsertBuffer(),
		nodestate:        observer.nodestate,
		overlayService:   observer.overlayService,
		rsStats:          make(map[string]*partialRSStats),
		repairOverrides:  observer.repairOverrides,
		nodeFailureRate:  observer.nodeFailureRate,
		getNodesEstimate: observer.getNodesEstimate,
		log:              observer.logger,
		doDeclumping:     observer.doDeclumping,
		doPlacementCheck: observer.doPlacementCheck,
		getObserverStats: observer.getObserverStats,
	}
}

func (fork *observerFork) getStatsByRS(redundancy storj.RedundancyScheme) *partialRSStats {
	rsString := getRSString(fork.loadRedundancy(redundancy))

	stats, ok := fork.rsStats[rsString]
	if !ok {
		observerStats := fork.getObserverStats(rsString)

		fork.rsStats[rsString] = &partialRSStats{
			iterationAggregates: aggregateStats{},
			segmentStats:        observerStats.segmentStats,
		}
		return fork.rsStats[rsString]
	}

	return stats
}

func (fork *observerFork) loadRedundancy(redundancy storj.RedundancyScheme) (int, int, int, int) {
	repair := int(redundancy.RepairShares)

	overrideValue := fork.repairOverrides.GetOverrideValue(redundancy)
	if overrideValue != 0 {
		repair = int(overrideValue)
	}

	return int(redundancy.RequiredShares), repair, int(redundancy.OptimalShares), int(redundancy.TotalShares)
}

// Process repair implementation of partial's Process.
func (fork *observerFork) Process(ctx context.Context, segments []rangedloop.Segment) (err error) {
	for _, segment := range segments {
		if err := fork.process(ctx, &segment); err != nil {
			return err
		}
	}

	return nil
}

func (fork *observerFork) process(ctx context.Context, segment *rangedloop.Segment) (err error) {
	if segment.Inline() {
		if fork.lastStreamID.Compare(segment.StreamID) != 0 {
			fork.lastStreamID = segment.StreamID
			fork.totalStats.objectsChecked++
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
		fork.totalStats.objectsChecked++
	}

	fork.totalStats.remoteSegmentsChecked++
	stats.iterationAggregates.remoteSegmentsChecked++

	// ensure we get values, even if only zero values, so that redash can have an alert based on this
	mon.Counter("checker_segments_below_min_req").Inc(0) //mon:locked
	pieces := segment.Pieces
	if len(pieces) == 0 {
		fork.log.Debug("no pieces on remote segment")
		return nil
	}

	totalNumNodes, err := fork.getNodesEstimate(ctx)
	if err != nil {
		return Error.New("could not get estimate of total number of nodes: %w", err)
	}

	missingPieces, err := fork.nodestate.MissingPieces(ctx, segment.CreatedAt, segment.Pieces)
	if err != nil {
		fork.totalStats.remoteSegmentsFailedToCheck++
		stats.iterationAggregates.remoteSegmentsFailedToCheck++
		return Error.New("error getting missing pieces: %w", err)
	}

	// reuse allNodeIDs slice if its large enough
	if cap(fork.allNodeIDs) < len(pieces) {
		fork.allNodeIDs = make([]storj.NodeID, len(pieces))
	} else {
		fork.allNodeIDs = fork.allNodeIDs[:len(pieces)]
	}

	for i, p := range pieces {
		fork.allNodeIDs[i] = p.StorageNode
	}

	var clumpedPieces metabase.Pieces
	var lastNets []string
	if fork.doDeclumping {
		// if multiple pieces are on the same last_net, keep only the first one. The rest are
		// to be considered retrievable but unhealthy.
		lastNets, err = fork.overlayService.GetNodesNetworkInOrder(ctx, fork.allNodeIDs)
		if err != nil {
			fork.totalStats.remoteSegmentsFailedToCheck++
			stats.iterationAggregates.remoteSegmentsFailedToCheck++
			return errs.Combine(Error.New("error determining node last_nets"), err)
		}
		clumpedPieces = repair.FindClumpedPieces(segment.Pieces, lastNets)
	}

	numPiecesOutOfPlacement := 0
	if fork.doPlacementCheck && segment.Placement != storj.EveryCountry {
		outOfPlacementNodes, err := fork.overlayService.GetNodesOutOfPlacement(ctx, fork.allNodeIDs, segment.Placement)
		if err != nil {
			fork.totalStats.remoteSegmentsFailedToCheck++
			stats.iterationAggregates.remoteSegmentsFailedToCheck++
			return errs.Combine(Error.New("error determining nodes placement"), err)
		}

		numPiecesOutOfPlacement = len(outOfPlacementNodes)
	}

	numHealthy := len(pieces) - len(missingPieces) - len(clumpedPieces)
	mon.IntVal("checker_segment_total_count").Observe(int64(len(pieces))) //mon:locked
	stats.segmentStats.segmentTotalCount.Observe(int64(len(pieces)))

	mon.IntVal("checker_segment_healthy_count").Observe(int64(numHealthy)) //mon:locked
	stats.segmentStats.segmentHealthyCount.Observe(int64(numHealthy))
	mon.IntVal("checker_segment_clumped_count").Observe(int64(len(clumpedPieces))) //mon:locked
	stats.segmentStats.segmentClumpedCount.Observe(int64(len(clumpedPieces)))
	mon.IntVal("checker_segment_off_placement_count").Observe(int64(numPiecesOutOfPlacement)) //mon:locked
	stats.segmentStats.segmentOffPlacementCount.Observe(int64(numPiecesOutOfPlacement))

	segmentAge := time.Since(segment.CreatedAt)
	mon.IntVal("checker_segment_age").Observe(int64(segmentAge.Seconds())) //mon:locked
	stats.segmentStats.segmentAge.Observe(int64(segmentAge.Seconds()))

	required, repairThreshold, successThreshold, _ := fork.loadRedundancy(segment.Redundancy)
	segmentHealth := repair.SegmentHealth(numHealthy, required, totalNumNodes, fork.nodeFailureRate)
	mon.FloatVal("checker_segment_health").Observe(segmentHealth) //mon:locked
	stats.segmentStats.segmentHealth.Observe(segmentHealth)

	// we repair when the number of healthy pieces is less than or equal to the repair threshold and is greater or equal to
	// minimum required pieces in redundancy
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing).
	// separate case is when we find pieces which are outside segment placement. in such case we are putting segment
	// into queue right away.
	if (numHealthy <= repairThreshold && numHealthy < successThreshold) || numPiecesOutOfPlacement > 0 {
		mon.FloatVal("checker_injured_segment_health").Observe(segmentHealth) //mon:locked
		stats.segmentStats.injuredSegmentHealth.Observe(segmentHealth)
		fork.totalStats.remoteSegmentsNeedingRepair++
		stats.iterationAggregates.remoteSegmentsNeedingRepair++
		err := fork.repairQueue.Insert(ctx, &queue.InjuredSegment{
			StreamID:      segment.StreamID,
			Position:      segment.Position,
			UpdatedAt:     time.Now().UTC(),
			SegmentHealth: segmentHealth,
		}, func() {
			// Counters are increased after the queue has determined
			// that the segment wasn't already queued for repair.
			fork.totalStats.newRemoteSegmentsNeedingRepair++
			stats.iterationAggregates.newRemoteSegmentsNeedingRepair++
		})
		if err != nil {
			fork.log.Error("error adding injured segment to queue", zap.Error(err))
			return nil
		}

		// monitor irreparable segments
		numRetrievable := len(pieces) - len(missingPieces)
		if numRetrievable < required {
			if !slices.Contains(fork.totalStats.objectsLost, segment.StreamID) {
				fork.totalStats.objectsLost = append(fork.totalStats.objectsLost, segment.StreamID)
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

			mon.IntVal("checker_segment_time_until_irreparable").Observe(int64(segmentAge.Seconds())) //mon:locked
			stats.segmentStats.segmentTimeUntilIrreparable.Observe(int64(segmentAge.Seconds()))

			fork.totalStats.remoteSegmentsLost++
			stats.iterationAggregates.remoteSegmentsLost++

			mon.Counter("checker_segments_below_min_req").Inc(1) //mon:locked
			stats.segmentStats.segmentsBelowMinReq.Inc(1)

			var unhealthyNodes []string
			for _, p := range missingPieces {
				unhealthyNodes = append(unhealthyNodes, p.StorageNode.String())
			}
			fork.log.Warn("checker found irreparable segment", zap.String("Segment StreamID", segment.StreamID.String()), zap.Int("Segment Position",
				int(segment.Position.Encode())), zap.Int("total pieces", len(pieces)), zap.Int("min required", required), zap.String("unhealthy node IDs", strings.Join(unhealthyNodes, ",")))
		} else if numRetrievable > repairThreshold {
			// This segment is to be repaired because of clumping (it wouldn't need repair yet
			// otherwise). Produce a brief report of where the clumping occurred so that we have
			// a better understanding of the cause.
			clumpedNets := clumpingReport{lastNets: lastNets}
			fork.log.Info("segment needs repair because of clumping", zap.Stringer("Segment StreamID", segment.StreamID), zap.Uint64("Segment Position", segment.Position.Encode()), zap.Int("total pieces", len(pieces)), zap.Int("min required", required), zap.Stringer("clumping", &clumpedNets))
		}
	} else {
		if numHealthy > repairThreshold && numHealthy <= (repairThreshold+len(fork.totalStats.remoteSegmentsOverThreshold)) {
			// record metrics for segments right above repair threshold
			// numHealthy=repairThreshold+1 through numHealthy=repairThreshold+5
			for i := range fork.totalStats.remoteSegmentsOverThreshold {
				if numHealthy == (repairThreshold + i + 1) {
					fork.totalStats.remoteSegmentsOverThreshold[i]++
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
			lastNet = "unknown"
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
