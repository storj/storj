// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/segmentloop"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair"
	"storj.io/storj/satellite/repair/queue"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("repair checker")
	mon   = monkit.Package()
)

// Checker contains the information needed to do checks for missing pieces.
//
// architecture: Chore
type Checker struct {
	logger               *zap.Logger
	repairQueue          queue.RepairQueue
	metabase             *metabase.DB
	segmentLoop          *segmentloop.Service
	nodestate            *ReliabilityCache
	overlayService       *overlay.Service
	statsCollector       *statsCollector
	repairOverrides      RepairOverridesMap
	nodeFailureRate      float64
	repairQueueBatchSize int
	Loop                 *sync2.Cycle
}

// NewChecker creates a new instance of checker.
func NewChecker(logger *zap.Logger, repairQueue queue.RepairQueue, metabase *metabase.DB, segmentLoop *segmentloop.Service, overlay *overlay.Service, config Config) *Checker {
	return &Checker{
		logger: logger,

		repairQueue:          repairQueue,
		metabase:             metabase,
		segmentLoop:          segmentLoop,
		nodestate:            NewReliabilityCache(overlay, config.ReliabilityCacheStaleness),
		overlayService:       overlay,
		statsCollector:       newStatsCollector(),
		repairOverrides:      config.RepairOverrides.GetMap(),
		nodeFailureRate:      config.NodeFailureRate,
		repairQueueBatchSize: config.RepairQueueInsertBatchSize,

		Loop: sync2.NewCycle(config.Interval),
	}
}

// Run the checker loop.
func (checker *Checker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return checker.Loop.Run(ctx, checker.IdentifyInjuredSegments)
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

func (checker *Checker) createInsertBuffer() *queue.InsertBuffer {
	return queue.NewInsertBuffer(checker.repairQueue, checker.repairQueueBatchSize)
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
		repairQueue:      checker.createInsertBuffer(),
		nodestate:        checker.nodestate,
		overlayService:   checker.overlayService,
		statsCollector:   checker.statsCollector,
		monStats:         aggregateStats{},
		repairOverrides:  checker.repairOverrides,
		nodeFailureRate:  checker.nodeFailureRate,
		getNodesEstimate: checker.getNodesEstimate,
		log:              checker.logger,
	}
	err = checker.segmentLoop.Join(ctx, observer)
	if err != nil {
		if !errs2.IsCanceled(err) {
			checker.logger.Error("IdentifyInjuredSegments error", zap.Error(err))
		}
		return nil
	}

	err = observer.repairQueue.Flush(ctx)
	if err != nil {
		return Error.Wrap(err)
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
	mon.IntVal("remote_files_lost").Observe(int64(len(observer.monStats.objectsLost)))                         //mon:locked
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

var _ segmentloop.Observer = (*checkerObserver)(nil)

// checkerObserver implements the metainfo loop Observer interface.
//
// architecture: Observer
type checkerObserver struct {
	repairQueue      *queue.InsertBuffer
	nodestate        *ReliabilityCache
	overlayService   *overlay.Service
	statsCollector   *statsCollector
	monStats         aggregateStats // TODO(cam): once we verify statsCollector reports data correctly, remove this
	repairOverrides  RepairOverridesMap
	nodeFailureRate  float64
	getNodesEstimate func(ctx context.Context) (int, error)
	log              *zap.Logger

	lastStreamID uuid.UUID
}

// NewCheckerObserver creates new checker observer instance.
func NewCheckerObserver(checker *Checker) segmentloop.Observer {
	return &checkerObserver{
		repairQueue:      checker.createInsertBuffer(),
		nodestate:        checker.nodestate,
		overlayService:   checker.overlayService,
		statsCollector:   checker.statsCollector,
		monStats:         aggregateStats{},
		repairOverrides:  checker.repairOverrides,
		nodeFailureRate:  checker.nodeFailureRate,
		getNodesEstimate: checker.getNodesEstimate,
		log:              checker.logger,
	}
}

// checks for a stream id in slice.
func containsStreamID(a []uuid.UUID, x uuid.UUID) bool {
	for _, n := range a {
		if bytes.Equal(x[:], n[:]) {
			return true
		}
	}
	return false
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

// LoopStarted is called at each start of a loop.
func (obs *checkerObserver) LoopStarted(context.Context, segmentloop.LoopInfo) (err error) {
	return nil
}

func (obs *checkerObserver) RemoteSegment(ctx context.Context, segment *segmentloop.Segment) (err error) {
	// we are explicitly not adding monitoring here as we are tracking loop observers separately

	// ignore segment if expired
	if segment.Expired(time.Now()) {
		return nil
	}

	stats := obs.getStatsByRS(segment.Redundancy)

	if obs.lastStreamID.Compare(segment.StreamID) != 0 {
		obs.lastStreamID = segment.StreamID
		stats.iterationAggregates.objectsChecked++

		obs.monStats.objectsChecked++
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

	totalNumNodes, err := obs.getNodesEstimate(ctx)
	if err != nil {
		return Error.New("could not get estimate of total number of nodes: %w", err)
	}

	missingPieces, err := obs.nodestate.MissingPieces(ctx, segment.CreatedAt, segment.Pieces)
	if err != nil {
		obs.monStats.remoteSegmentsFailedToCheck++
		stats.iterationAggregates.remoteSegmentsFailedToCheck++
		return errs.Combine(Error.New("error getting missing pieces"), err)
	}

	// if multiple pieces are on the same last_net, keep only the first one. The rest are
	// to be considered retrievable but unhealthy.
	nodeIDs := make([]storj.NodeID, len(pieces))
	for i, p := range pieces {
		nodeIDs[i] = p.StorageNode
	}
	lastNets, err := obs.overlayService.GetNodesNetworkInOrder(ctx, nodeIDs)
	if err != nil {
		obs.monStats.remoteSegmentsFailedToCheck++
		stats.iterationAggregates.remoteSegmentsFailedToCheck++
		return errs.Combine(Error.New("error determining node last_nets"), err)
	}
	clumpedPieces := repair.FindClumpedPieces(segment.Pieces, lastNets)

	numHealthy := len(pieces) - len(missingPieces) - len(clumpedPieces)
	mon.IntVal("checker_segment_total_count").Observe(int64(len(pieces))) //mon:locked
	stats.segmentTotalCount.Observe(int64(len(pieces)))
	mon.IntVal("checker_segment_healthy_count").Observe(int64(numHealthy)) //mon:locked
	stats.segmentHealthyCount.Observe(int64(numHealthy))
	mon.IntVal("checker_segment_clumped_count").Observe(int64(len(clumpedPieces))) //mon:locked
	stats.segmentClumpedCount.Observe(int64(len(clumpedPieces)))

	segmentAge := time.Since(segment.CreatedAt)
	mon.IntVal("checker_segment_age").Observe(int64(segmentAge.Seconds())) //mon:locked
	stats.segmentAge.Observe(int64(segmentAge.Seconds()))

	required, repairThreshold, successThreshold, _ := obs.loadRedundancy(segment.Redundancy)

	segmentHealth := repair.SegmentHealth(numHealthy, required, totalNumNodes, obs.nodeFailureRate)
	mon.FloatVal("checker_segment_health").Observe(segmentHealth) //mon:locked
	stats.segmentHealth.Observe(segmentHealth)

	// we repair when the number of healthy pieces is less than or equal to the repair threshold and is greater or equal to
	// minimum required pieces in redundancy
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	if numHealthy <= repairThreshold && numHealthy < successThreshold {
		mon.FloatVal("checker_injured_segment_health").Observe(segmentHealth) //mon:locked
		stats.injuredSegmentHealth.Observe(segmentHealth)
		obs.monStats.remoteSegmentsNeedingRepair++
		stats.iterationAggregates.remoteSegmentsNeedingRepair++
		err := obs.repairQueue.Insert(ctx, &queue.InjuredSegment{
			StreamID:      segment.StreamID,
			Position:      segment.Position,
			UpdatedAt:     time.Now().UTC(),
			SegmentHealth: segmentHealth,
		}, func() {
			// Counters are increased after the queue has determined
			// that the segment wasn't already queued for repair.
			obs.monStats.newRemoteSegmentsNeedingRepair++
			stats.iterationAggregates.newRemoteSegmentsNeedingRepair++
		})
		if err != nil {
			obs.log.Error("error adding injured segment to queue", zap.Error(err))
			return nil
		}

		// monitor irreperable segments
		if numHealthy < required {
			if !containsStreamID(obs.monStats.objectsLost, segment.StreamID) {
				obs.monStats.objectsLost = append(obs.monStats.objectsLost, segment.StreamID)
			}
			if !containsStreamID(stats.iterationAggregates.objectsLost, segment.StreamID) {
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
			stats.segmentTimeUntilIrreparable.Observe(int64(segmentAge.Seconds()))

			obs.monStats.remoteSegmentsLost++
			stats.iterationAggregates.remoteSegmentsLost++

			mon.Counter("checker_segments_below_min_req").Inc(1) //mon:locked
			stats.segmentsBelowMinReq.Inc(1)
			var unhealthyNodes []string
			for _, p := range missingPieces {
				unhealthyNodes = append(unhealthyNodes, p.StorageNode.String())
			}
			obs.log.Warn("checker found irreparable segment", zap.String("Segment StreamID", segment.StreamID.String()), zap.Int("Segment Position",
				int(segment.Position.Encode())), zap.Int("total pieces", len(pieces)), zap.Int("min required", required), zap.String("unhealthy node IDs", strings.Join(unhealthyNodes, ",")))
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

func (obs *checkerObserver) InlineSegment(ctx context.Context, segment *segmentloop.Segment) (err error) {
	// inline segments are not repaired but we would like to count as checked also
	// objects that have only inline segments
	if obs.lastStreamID.Compare(segment.StreamID) != 0 {
		obs.monStats.objectsChecked++
	}
	return nil
}
