// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/segmentloop"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair"
	"storj.io/storj/satellite/repair/queue"
)

var _ rangedloop.Observer = (*RangedLoopObserver)(nil)

// RangedLoopObserver implements the ranged loop Observer interface.  Should be renamed to checkerObserver after rangedloop will replace segmentloop.
//
// architecture: Observer
type RangedLoopObserver struct {
	logger               *zap.Logger
	repairQueue          queue.RepairQueue
	nodestate            *ReliabilityCache
	statsCollector       *statsCollector
	repairOverrides      RepairOverridesMap
	nodeFailureRate      float64
	repairQueueBatchSize int
	TotalStats           aggregateStats
}

// NewRangedLoopObserver creates new checker observer instance.
func NewRangedLoopObserver(logger *zap.Logger, repairQueue queue.RepairQueue, overlay *overlay.Service, config Config) rangedloop.Observer {
	return &RangedLoopObserver{
		logger: logger,

		repairQueue:          repairQueue,
		nodestate:            NewReliabilityCache(overlay, config.ReliabilityCacheStaleness),
		statsCollector:       newStatsCollector(),
		repairOverrides:      config.RepairOverrides.GetMap(),
		nodeFailureRate:      config.NodeFailureRate,
		repairQueueBatchSize: config.RepairQueueInsertBatchSize,

		TotalStats: aggregateStats{},
	}
}

// getNodesEstimate updates the estimate of the total number of nodes. It is guaranteed
// to return a number greater than 0 when the error is nil.
//
// We can't calculate this upon first starting a Ranged Loop Observer, because there may not be any
// nodes yet. We expect that there will be nodes before there are segments, though.
func (observer *RangedLoopObserver) getNodesEstimate(ctx context.Context) (int, error) {
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

func (observer *RangedLoopObserver) createInsertBuffer() *queue.InsertBuffer {
	return queue.NewInsertBuffer(observer.repairQueue, observer.repairQueueBatchSize)
}

// Start starts parallel segments loop.
func (observer *RangedLoopObserver) Start(ctx context.Context, startTime time.Time) error {
	return nil
}

// Fork creates a Partial to process a chunk of all the segments.
func (observer *RangedLoopObserver) Fork(ctx context.Context) (rangedloop.Partial, error) {
	return NewRangedLoopCheckerPartial(observer), nil
}

// Join is called after the chunk for Partial is done.
// This gives the opportunity to merge the output like in a reduce step.
func (observer *RangedLoopObserver) Join(ctx context.Context, partial rangedloop.Partial) error {
	repPartial := partial.(*repairPartial)
	if err := repPartial.repairQueue.Flush(ctx); err != nil {
		return Error.Wrap(err)
	}

	observer.TotalStats.objectsLost = append(observer.TotalStats.objectsLost, repPartial.monStats.objectsLost...)
	observer.TotalStats.newRemoteSegmentsNeedingRepair += repPartial.monStats.newRemoteSegmentsNeedingRepair
	observer.TotalStats.remoteSegmentsChecked += repPartial.monStats.remoteSegmentsChecked
	observer.TotalStats.remoteSegmentsLost += repPartial.monStats.remoteSegmentsLost
	observer.TotalStats.remoteSegmentsFailedToCheck += repPartial.monStats.remoteSegmentsFailedToCheck
	observer.TotalStats.remoteSegmentsNeedingRepair += repPartial.monStats.remoteSegmentsNeedingRepair
	observer.TotalStats.objectsChecked += repPartial.monStats.objectsChecked

	return nil
}

// Finish is called after all segments are processed by all observers.
func (observer *RangedLoopObserver) Finish(ctx context.Context) error {
	startTime := time.Now()

	if err := observer.createInsertBuffer().Flush(ctx); err != nil {
		return Error.Wrap(err)
	}

	// remove all segments which were not seen as unhealthy by this checker iteration
	healthyDeleted, err := observer.repairQueue.Clean(ctx, startTime)
	if err != nil {
		return Error.Wrap(err)
	}
	observer.statsCollector.collectAggregates()
	mon.IntVal("remote_files_checked").Observe(observer.TotalStats.objectsChecked)                               //mon:locked
	mon.IntVal("remote_segments_checked").Observe(observer.TotalStats.remoteSegmentsChecked)                     //mon:locked
	mon.IntVal("remote_segments_failed_to_check").Observe(observer.TotalStats.remoteSegmentsFailedToCheck)       //mon:locked
	mon.IntVal("remote_segments_needing_repair").Observe(observer.TotalStats.remoteSegmentsNeedingRepair)        //mon:locked
	mon.IntVal("new_remote_segments_needing_repair").Observe(observer.TotalStats.newRemoteSegmentsNeedingRepair) //mon:locked
	mon.IntVal("remote_segments_lost").Observe(observer.TotalStats.remoteSegmentsLost)                           //mon:locked
	mon.IntVal("remote_files_lost").Observe(int64(len(observer.TotalStats.objectsLost)))                         //mon:locked
	mon.IntVal("healthy_segments_removed_from_queue").Observe(healthyDeleted)                                    //mon:locked
	allUnhealthy := observer.TotalStats.remoteSegmentsNeedingRepair + observer.TotalStats.remoteSegmentsFailedToCheck
	allChecked := observer.TotalStats.remoteSegmentsChecked
	allHealthy := allChecked - allUnhealthy
	mon.FloatVal("remote_segments_healthy_percentage").Observe(100 * float64(allHealthy) / float64(allChecked)) //mon:locked
	return nil
}

// repairPartial implements the ranged loop Partial interface.
//
// architecture: Observer
type repairPartial struct {
	repairQueue      *queue.InsertBuffer
	nodestate        *ReliabilityCache
	statsCollector   *statsCollector
	monStats         aggregateStats // TODO(cam): once we verify statsCollector reports data correctly, remove this
	repairOverrides  RepairOverridesMap
	nodeFailureRate  float64
	getNodesEstimate func(ctx context.Context) (int, error)
	log              *zap.Logger
	lastStreamID     uuid.UUID
}

// NewRangedLoopCheckerPartial creates new checker partial instance.
func NewRangedLoopCheckerPartial(observer *RangedLoopObserver) rangedloop.Partial {
	// we can only share thread-safe objects.
	return &repairPartial{
		repairQueue:      observer.createInsertBuffer(),
		nodestate:        observer.nodestate,
		statsCollector:   newStatsCollector(),
		monStats:         aggregateStats{},
		repairOverrides:  observer.repairOverrides,
		nodeFailureRate:  observer.nodeFailureRate,
		getNodesEstimate: observer.getNodesEstimate,
		log:              observer.logger,
	}
}

func (cp *repairPartial) getStatsByRS(redundancy storj.RedundancyScheme) *stats {
	rsString := getRSString(cp.loadRedundancy(redundancy))
	return cp.statsCollector.getStatsByRS(rsString)
}

func (cp *repairPartial) loadRedundancy(redundancy storj.RedundancyScheme) (int, int, int, int) {
	repair := int(redundancy.RepairShares)

	overrideValue := cp.repairOverrides.GetOverrideValue(redundancy)
	if overrideValue != 0 {
		repair = int(overrideValue)
	}

	return int(redundancy.RequiredShares), repair, int(redundancy.OptimalShares), int(redundancy.TotalShares)
}

// Process repair implementation of partial's Process.
func (cp *repairPartial) Process(ctx context.Context, segments []segmentloop.Segment) (errors error) {
	for _, segment := range segments {
		if segment.Inline() {
			if cp.lastStreamID.Compare(segment.StreamID) != 0 {
				cp.monStats.objectsChecked++
			}

			continue
		}

		// ignore segment if expired
		if segment.Expired(time.Now()) {
			continue
		}

		stats := cp.getStatsByRS(segment.Redundancy)
		if cp.lastStreamID.Compare(segment.StreamID) != 0 {
			cp.lastStreamID = segment.StreamID
			stats.iterationAggregates.objectsChecked++
			cp.monStats.objectsChecked++
		}

		cp.monStats.remoteSegmentsChecked++
		stats.iterationAggregates.remoteSegmentsChecked++

		// ensure we get values, even if only zero values, so that redash can have an alert based on this
		mon.Counter("checker_segments_below_min_req").Inc(0) //mon:locked
		stats.segmentsBelowMinReq.Inc(0)
		pieces := segment.Pieces
		if len(pieces) == 0 {
			cp.log.Debug("no pieces on remote segment")
			continue
		}

		totalNumNodes, err := cp.getNodesEstimate(ctx)
		if err != nil {
			errors = errs.Combine(errors, Error.New("could not get estimate of total number of nodes: %w", err))
		}

		missingPieces, err := cp.nodestate.MissingPieces(ctx, segment.CreatedAt, segment.Pieces)
		if err != nil {
			cp.monStats.remoteSegmentsFailedToCheck++
			stats.iterationAggregates.remoteSegmentsFailedToCheck++
			errors = errs.Combine(errors, Error.New("error getting missing pieces"), err)
		}

		numHealthy := len(pieces) - len(missingPieces)
		mon.IntVal("checker_segment_total_count").Observe(int64(len(pieces))) //mon:locked
		stats.segmentTotalCount.Observe(int64(len(pieces)))

		mon.IntVal("checker_segment_healthy_count").Observe(int64(numHealthy)) //mon:locked
		stats.segmentHealthyCount.Observe(int64(numHealthy))

		segmentAge := time.Since(segment.CreatedAt)
		mon.IntVal("checker_segment_age").Observe(int64(segmentAge.Seconds())) //mon:locked
		stats.segmentAge.Observe(int64(segmentAge.Seconds()))

		required, repairThreshold, successThreshold, _ := cp.loadRedundancy(segment.Redundancy)
		segmentHealth := repair.SegmentHealth(numHealthy, required, totalNumNodes, cp.nodeFailureRate)
		mon.FloatVal("checker_segment_health").Observe(segmentHealth) //mon:locked
		stats.segmentHealth.Observe(segmentHealth)

		// we repair when the number of healthy pieces is less than or equal to the repair threshold and is greater or equal to
		// minimum required pieces in redundancy
		// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
		if numHealthy <= repairThreshold && numHealthy < successThreshold {
			mon.FloatVal("checker_injured_segment_health").Observe(segmentHealth) //mon:locked
			stats.injuredSegmentHealth.Observe(segmentHealth)
			cp.monStats.remoteSegmentsNeedingRepair++
			stats.iterationAggregates.remoteSegmentsNeedingRepair++
			err := cp.repairQueue.Insert(ctx, &queue.InjuredSegment{
				StreamID:      segment.StreamID,
				Position:      segment.Position,
				UpdatedAt:     time.Now().UTC(),
				SegmentHealth: segmentHealth,
			}, func() {
				// Counters are increased after the queue has determined
				// that the segment wasn't already queued for repair.
				cp.monStats.newRemoteSegmentsNeedingRepair++
				stats.iterationAggregates.newRemoteSegmentsNeedingRepair++
			})
			if err != nil {
				cp.log.Error("error adding injured segment to queue", zap.Error(err))
				continue
			}

			// monitor irreparable segments
			if numHealthy < required {
				if !containsStreamID(cp.monStats.objectsLost, segment.StreamID) {
					cp.monStats.objectsLost = append(cp.monStats.objectsLost, segment.StreamID)
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

				cp.monStats.remoteSegmentsLost++
				stats.iterationAggregates.remoteSegmentsLost++

				mon.Counter("checker_segments_below_min_req").Inc(1) //mon:locked
				stats.segmentsBelowMinReq.Inc(1)

				var unhealthyNodes []string
				for _, p := range missingPieces {
					unhealthyNodes = append(unhealthyNodes, p.StorageNode.String())
				}
				cp.log.Warn("checker found irreparable segment", zap.String("Segment StreamID", segment.StreamID.String()), zap.Int("Segment Position",
					int(segment.Position.Encode())), zap.Int("total pieces", len(pieces)), zap.Int("min required", required), zap.String("unhealthy node IDs", strings.Join(unhealthyNodes, ",")))
			}
		} else {
			if numHealthy > repairThreshold && numHealthy <= (repairThreshold+len(cp.monStats.remoteSegmentsOverThreshold)) {
				// record metrics for segments right above repair threshold
				// numHealthy=repairThreshold+1 through numHealthy=repairThreshold+5
				for i := range cp.monStats.remoteSegmentsOverThreshold {
					if numHealthy == (repairThreshold + i + 1) {
						cp.monStats.remoteSegmentsOverThreshold[i]++
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

		continue
	}

	mon.IntVal("remote_segments_over_threshold_1").Observe(cp.monStats.remoteSegmentsOverThreshold[0]) //mon:locked
	mon.IntVal("remote_segments_over_threshold_2").Observe(cp.monStats.remoteSegmentsOverThreshold[1]) //mon:locked
	mon.IntVal("remote_segments_over_threshold_3").Observe(cp.monStats.remoteSegmentsOverThreshold[2]) //mon:locked
	mon.IntVal("remote_segments_over_threshold_4").Observe(cp.monStats.remoteSegmentsOverThreshold[3]) //mon:locked
	mon.IntVal("remote_segments_over_threshold_5").Observe(cp.monStats.remoteSegmentsOverThreshold[4]) //mon:locked

	return nil
}
