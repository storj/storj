// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
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
	startTime            time.Time
	TotalStats           aggregateStats

	// Remote segments over threshold counters, to avoid panics during parallel Processing.
	counter1 *monkit.Counter
	counter2 *monkit.Counter
	counter3 *monkit.Counter
	counter4 *monkit.Counter
	counter5 *monkit.Counter
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

		counter1: mon.Counter("rl_remote_segments_over_threshold_1"),
		counter2: mon.Counter("rl_remote_segments_over_threshold_2"),
		counter3: mon.Counter("rl_remote_segments_over_threshold_3"),
		counter4: mon.Counter("rl_remote_segments_over_threshold_4"),
		counter5: mon.Counter("rl_remote_segments_over_threshold_5"),
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

// TestingCompareInjuredSegmentIDs compares stream id of injured segment.
func (observer *RangedLoopObserver) TestingCompareInjuredSegmentIDs(ctx context.Context, streamIDs []uuid.UUID) error {
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
func (observer *RangedLoopObserver) Start(ctx context.Context, startTime time.Time) error {
	observer.counter1.Reset() // reset counter values before new ranged loop run
	observer.counter2.Reset()
	observer.counter3.Reset()
	observer.counter4.Reset()
	observer.counter5.Reset()

	observer.TotalStats = aggregateStats{}
	observer.startTime = startTime
	return nil
}

// Fork creates a Partial to process a chunk of all the segments.
func (observer *RangedLoopObserver) Fork(ctx context.Context) (rangedloop.Partial, error) {
	return newRangedLoopCheckerPartial(observer), nil
}

// Join is called after the chunk for Partial is done.
// This gives the opportunity to merge the output like in a reduce step.
func (observer *RangedLoopObserver) Join(ctx context.Context, partial rangedloop.Partial) error {
	repPartial, ok := partial.(*repairPartial)
	if !ok {
		return Error.New("expected partial type %T but got %T", repPartial, partial)
	}

	observer.statsCollector.combineCollectors(repPartial.statsCollector)

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

	observer.counter1.Inc(repPartial.monStats.remoteSegmentsOverThreshold[0]) //mon:locked
	observer.counter2.Inc(repPartial.monStats.remoteSegmentsOverThreshold[1]) //mon:locked
	observer.counter3.Inc(repPartial.monStats.remoteSegmentsOverThreshold[2]) //mon:locked
	observer.counter4.Inc(repPartial.monStats.remoteSegmentsOverThreshold[3]) //mon:locked
	observer.counter5.Inc(repPartial.monStats.remoteSegmentsOverThreshold[4]) //mon:locked

	return nil
}

// Finish is called after all segments are processed by all observers.
func (observer *RangedLoopObserver) Finish(ctx context.Context) error {
	// remove all segments which were not seen as unhealthy by this checker iteration
	healthyDeleted, err := observer.repairQueue.Clean(ctx, observer.startTime)
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

// newRangedLoopCheckerPartial creates new checker partial instance.
func newRangedLoopCheckerPartial(observer *RangedLoopObserver) rangedloop.Partial {
	// we can only share thread-safe objects.
	return &repairPartial{
		repairQueue:      observer.createInsertBuffer(),
		nodestate:        observer.nodestate,
		statsCollector:   newStatsCollector(),
		repairOverrides:  observer.repairOverrides,
		nodeFailureRate:  observer.nodeFailureRate,
		getNodesEstimate: observer.getNodesEstimate,
		log:              observer.logger,
	}
}

func (rp *repairPartial) getStatsByRS(redundancy storj.RedundancyScheme) *stats {
	rsString := getRSString(rp.loadRedundancy(redundancy))
	return rp.statsCollector.getStatsByRS(rsString)
}

func (rp *repairPartial) loadRedundancy(redundancy storj.RedundancyScheme) (int, int, int, int) {
	repair := int(redundancy.RepairShares)

	overrideValue := rp.repairOverrides.GetOverrideValue(redundancy)
	if overrideValue != 0 {
		repair = int(overrideValue)
	}

	return int(redundancy.RequiredShares), repair, int(redundancy.OptimalShares), int(redundancy.TotalShares)
}

// Process repair implementation of partial's Process.
func (rp *repairPartial) Process(ctx context.Context, segments []segmentloop.Segment) (errors error) {
	for _, segment := range segments {
		if segment.Inline() {
			if rp.lastStreamID.Compare(segment.StreamID) != 0 {
				rp.monStats.objectsChecked++
			}

			continue
		}

		// ignore segment if expired
		if segment.Expired(time.Now()) {
			continue
		}

		stats := rp.getStatsByRS(segment.Redundancy)
		if rp.lastStreamID.Compare(segment.StreamID) != 0 {
			rp.lastStreamID = segment.StreamID
			stats.iterationAggregates.objectsChecked++
			rp.monStats.objectsChecked++
		}

		rp.monStats.remoteSegmentsChecked++
		stats.iterationAggregates.remoteSegmentsChecked++

		// ensure we get values, even if only zero values, so that redash can have an alert based on this
		mon.Counter("checker_segments_below_min_req").Inc(0) //mon:locked
		stats.segmentsBelowMinReq.Inc(0)
		pieces := segment.Pieces
		if len(pieces) == 0 {
			rp.log.Debug("no pieces on remote segment")
			continue
		}

		totalNumNodes, err := rp.getNodesEstimate(ctx)
		if err != nil {
			errors = errs.Combine(errors, Error.New("could not get estimate of total number of nodes: %w", err))
		}

		missingPieces, err := rp.nodestate.MissingPieces(ctx, segment.CreatedAt, segment.Pieces)
		if err != nil {
			rp.monStats.remoteSegmentsFailedToCheck++
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

		required, repairThreshold, successThreshold, _ := rp.loadRedundancy(segment.Redundancy)
		segmentHealth := repair.SegmentHealth(numHealthy, required, totalNumNodes, rp.nodeFailureRate)
		mon.FloatVal("checker_segment_health").Observe(segmentHealth) //mon:locked
		stats.segmentHealth.Observe(segmentHealth)

		// we repair when the number of healthy pieces is less than or equal to the repair threshold and is greater or equal to
		// minimum required pieces in redundancy
		// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
		if numHealthy <= repairThreshold && numHealthy < successThreshold {
			mon.FloatVal("checker_injured_segment_health").Observe(segmentHealth) //mon:locked
			stats.injuredSegmentHealth.Observe(segmentHealth)
			rp.monStats.remoteSegmentsNeedingRepair++
			stats.iterationAggregates.remoteSegmentsNeedingRepair++
			err := rp.repairQueue.Insert(ctx, &queue.InjuredSegment{
				StreamID:      segment.StreamID,
				Position:      segment.Position,
				UpdatedAt:     time.Now().UTC(),
				SegmentHealth: segmentHealth,
			}, func() {
				// Counters are increased after the queue has determined
				// that the segment wasn't already queued for repair.
				rp.monStats.newRemoteSegmentsNeedingRepair++
				stats.iterationAggregates.newRemoteSegmentsNeedingRepair++
			})
			if err != nil {
				rp.log.Error("error adding injured segment to queue", zap.Error(err))
				continue
			}

			// monitor irreparable segments
			if numHealthy < required {
				if !containsStreamID(rp.monStats.objectsLost, segment.StreamID) {
					rp.monStats.objectsLost = append(rp.monStats.objectsLost, segment.StreamID)
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

				rp.monStats.remoteSegmentsLost++
				stats.iterationAggregates.remoteSegmentsLost++

				mon.Counter("checker_segments_below_min_req").Inc(1) //mon:locked
				stats.segmentsBelowMinReq.Inc(1)

				var unhealthyNodes []string
				for _, p := range missingPieces {
					unhealthyNodes = append(unhealthyNodes, p.StorageNode.String())
				}
				rp.log.Warn("checker found irreparable segment", zap.String("Segment StreamID", segment.StreamID.String()), zap.Int("Segment Position",
					int(segment.Position.Encode())), zap.Int("total pieces", len(pieces)), zap.Int("min required", required), zap.String("unhealthy node IDs", strings.Join(unhealthyNodes, ",")))
			}
		} else {
			if numHealthy > repairThreshold && numHealthy <= (repairThreshold+len(rp.monStats.remoteSegmentsOverThreshold)) {
				// record metrics for segments right above repair threshold
				// numHealthy=repairThreshold+1 through numHealthy=repairThreshold+5
				for i := range rp.monStats.remoteSegmentsOverThreshold {
					if numHealthy == (repairThreshold + i + 1) {
						rp.monStats.remoteSegmentsOverThreshold[i]++
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

	return nil
}
