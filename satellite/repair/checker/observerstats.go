// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"fmt"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/uuid"
)

type observerRSStats struct {
	// iterationAggregates contains the aggregated counters across all partials.
	// The values are observed by the distributions in iterationStats
	iterationAggregates aggregateStats

	// iterationStats are the distributions for per-iteration stats. The distributions
	// are updated using iterationAggregates after each loop iteration completes.
	iterationStats iterationRSStats

	// segmentStats contains threadsafe distributions and is shared by all partials. The
	// distributions are updated when processing the segment.
	segmentStats *segmentRSStats
}

// Stats implements the monkit.StatSource interface.
func (stats *observerRSStats) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	stats.iterationStats.objectsChecked.Stats(cb)
	stats.iterationStats.remoteSegmentsChecked.Stats(cb)
	stats.iterationStats.remoteSegmentsNeedingRepair.Stats(cb)
	stats.iterationStats.newRemoteSegmentsNeedingRepair.Stats(cb)
	stats.iterationStats.remoteSegmentsLost.Stats(cb)
	stats.iterationStats.objectsLost.Stats(cb)
	stats.iterationStats.remoteSegmentsFailedToCheck.Stats(cb)
	stats.iterationStats.remoteSegmentsHealthyPercentage.Stats(cb)

	stats.iterationStats.remoteSegmentsOverThreshold1.Stats(cb)
	stats.iterationStats.remoteSegmentsOverThreshold2.Stats(cb)
	stats.iterationStats.remoteSegmentsOverThreshold3.Stats(cb)
	stats.iterationStats.remoteSegmentsOverThreshold4.Stats(cb)
	stats.iterationStats.remoteSegmentsOverThreshold5.Stats(cb)

	stats.segmentStats.segmentsBelowMinReq.Stats(cb)
	stats.segmentStats.segmentTotalCount.Stats(cb)
	stats.segmentStats.segmentHealthyCount.Stats(cb)
	stats.segmentStats.segmentAge.Stats(cb)
	stats.segmentStats.segmentHealth.Stats(cb)
	stats.segmentStats.injuredSegmentHealth.Stats(cb)
	stats.segmentStats.segmentTimeUntilIrreparable.Stats(cb)
}

type iterationRSStats struct {
	objectsChecked                  *monkit.IntVal
	remoteSegmentsChecked           *monkit.IntVal
	remoteSegmentsNeedingRepair     *monkit.IntVal
	newRemoteSegmentsNeedingRepair  *monkit.IntVal
	remoteSegmentsLost              *monkit.IntVal
	objectsLost                     *monkit.IntVal
	remoteSegmentsFailedToCheck     *monkit.IntVal
	remoteSegmentsHealthyPercentage *monkit.FloatVal

	// remoteSegmentsOverThreshold[0]=# of healthy=rt+1, remoteSegmentsOverThreshold[1]=# of healthy=rt+2, etc...
	remoteSegmentsOverThreshold1 *monkit.IntVal
	remoteSegmentsOverThreshold2 *monkit.IntVal
	remoteSegmentsOverThreshold3 *monkit.IntVal
	remoteSegmentsOverThreshold4 *monkit.IntVal
	remoteSegmentsOverThreshold5 *monkit.IntVal
}

func newIterationRSStats(rs string) iterationRSStats {
	return iterationRSStats{
		objectsChecked:                  monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "remote_objects_checked").WithTag("rs_scheme", rs)),
		remoteSegmentsChecked:           monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "remote_segments_checked").WithTag("rs_scheme", rs)),
		remoteSegmentsNeedingRepair:     monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "remote_segments_needing_repair").WithTag("rs_scheme", rs)),
		newRemoteSegmentsNeedingRepair:  monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "new_remote_segments_needing_repair").WithTag("rs_scheme", rs)),
		remoteSegmentsLost:              monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "remote_segments_lost").WithTag("rs_scheme", rs)),
		objectsLost:                     monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "objects_lost").WithTag("rs_scheme", rs)),
		remoteSegmentsFailedToCheck:     monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "remote_segments_failed_to_check").WithTag("rs_scheme", rs)),
		remoteSegmentsHealthyPercentage: monkit.NewFloatVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "remote_segments_healthy_percentage").WithTag("rs_scheme", rs)),
		remoteSegmentsOverThreshold1:    monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "remote_segments_over_threshold_1").WithTag("rs_scheme", rs)),
		remoteSegmentsOverThreshold2:    monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "remote_segments_over_threshold_2").WithTag("rs_scheme", rs)),
		remoteSegmentsOverThreshold3:    monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "remote_segments_over_threshold_3").WithTag("rs_scheme", rs)),
		remoteSegmentsOverThreshold4:    monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "remote_segments_over_threshold_4").WithTag("rs_scheme", rs)),
		remoteSegmentsOverThreshold5:    monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "remote_segments_over_threshold_5").WithTag("rs_scheme", rs)),
	}
}

type partialRSStats struct {
	// iterationAggregates are counts aggregated by each partial for stats for the whole loop
	// and are aggregated into the observer during join. These aggregated counters
	// are tallied into distributions at the end of each loop.
	iterationAggregates aggregateStats

	// segmentStats contains thread-safe distributions and is shared by all partials. The
	// distributions are updated when processing the segment.
	segmentStats *segmentRSStats
}

type segmentRSStats struct {
	segmentsBelowMinReq         *monkit.Counter
	segmentTotalCount           *monkit.IntVal
	segmentHealthyCount         *monkit.IntVal
	segmentClumpedCount         *monkit.IntVal
	segmentOffPlacementCount    *monkit.IntVal
	segmentAge                  *monkit.IntVal
	segmentHealth               *monkit.FloatVal
	injuredSegmentHealth        *monkit.FloatVal
	segmentTimeUntilIrreparable *monkit.IntVal
}

func newSegmentRSStats(rs string) *segmentRSStats {
	return &segmentRSStats{
		segmentsBelowMinReq:         monkit.NewCounter(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segments_below_min_req").WithTag("rs_scheme", rs)),
		segmentTotalCount:           monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_total_count").WithTag("rs_scheme", rs)),
		segmentHealthyCount:         monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_healthy_count").WithTag("rs_scheme", rs)),
		segmentClumpedCount:         monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_clumped_count").WithTag("rs_scheme", rs)),
		segmentOffPlacementCount:    monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_off_placement_count").WithTag("rs_scheme", rs)),
		segmentAge:                  monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_age").WithTag("rs_scheme", rs)),
		segmentHealth:               monkit.NewFloatVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_health").WithTag("rs_scheme", rs)),
		injuredSegmentHealth:        monkit.NewFloatVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_injured_segment_health").WithTag("rs_scheme", rs)),
		segmentTimeUntilIrreparable: monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_time_until_irreparable").WithTag("rs_scheme", rs)),
	}
}

func (stats *observerRSStats) collectAggregates() {
	stats.iterationStats.objectsChecked.Observe(stats.iterationAggregates.objectsChecked)
	stats.iterationStats.remoteSegmentsChecked.Observe(stats.iterationAggregates.remoteSegmentsChecked)
	stats.iterationStats.remoteSegmentsNeedingRepair.Observe(stats.iterationAggregates.remoteSegmentsNeedingRepair)
	stats.iterationStats.newRemoteSegmentsNeedingRepair.Observe(stats.iterationAggregates.newRemoteSegmentsNeedingRepair)
	stats.iterationStats.remoteSegmentsLost.Observe(stats.iterationAggregates.remoteSegmentsLost)
	stats.iterationStats.objectsLost.Observe(int64(len(stats.iterationAggregates.objectsLost)))
	stats.iterationStats.remoteSegmentsFailedToCheck.Observe(stats.iterationAggregates.remoteSegmentsFailedToCheck)
	stats.iterationStats.remoteSegmentsOverThreshold1.Observe(stats.iterationAggregates.remoteSegmentsOverThreshold[0])
	stats.iterationStats.remoteSegmentsOverThreshold2.Observe(stats.iterationAggregates.remoteSegmentsOverThreshold[1])
	stats.iterationStats.remoteSegmentsOverThreshold3.Observe(stats.iterationAggregates.remoteSegmentsOverThreshold[2])
	stats.iterationStats.remoteSegmentsOverThreshold4.Observe(stats.iterationAggregates.remoteSegmentsOverThreshold[3])
	stats.iterationStats.remoteSegmentsOverThreshold5.Observe(stats.iterationAggregates.remoteSegmentsOverThreshold[4])

	allUnhealthy := stats.iterationAggregates.remoteSegmentsNeedingRepair + stats.iterationAggregates.remoteSegmentsFailedToCheck
	allChecked := stats.iterationAggregates.remoteSegmentsChecked
	allHealthy := allChecked - allUnhealthy

	stats.iterationStats.remoteSegmentsHealthyPercentage.Observe(100 * float64(allHealthy) / float64(allChecked))

	// resetting iteration aggregates after loop run finished
	stats.iterationAggregates = aggregateStats{}
}

// aggregateStats tallies data over the full checker iteration.
type aggregateStats struct {
	objectsChecked                 int64
	remoteSegmentsChecked          int64
	remoteSegmentsNeedingRepair    int64
	newRemoteSegmentsNeedingRepair int64
	remoteSegmentsLost             int64
	remoteSegmentsFailedToCheck    int64
	objectsLost                    []uuid.UUID

	// remoteSegmentsOverThreshold[0]=# of healthy=rt+1, remoteSegmentsOverThreshold[1]=# of healthy=rt+2, etc...
	remoteSegmentsOverThreshold [5]int64
}

func (a *aggregateStats) combine(stats aggregateStats) {
	a.objectsChecked += stats.objectsChecked
	a.remoteSegmentsChecked += stats.remoteSegmentsChecked
	a.remoteSegmentsNeedingRepair += stats.remoteSegmentsNeedingRepair
	a.newRemoteSegmentsNeedingRepair += stats.newRemoteSegmentsNeedingRepair
	a.remoteSegmentsLost += stats.remoteSegmentsLost
	a.remoteSegmentsFailedToCheck += stats.remoteSegmentsFailedToCheck
	a.objectsLost = append(a.objectsLost, stats.objectsLost...)

	a.remoteSegmentsOverThreshold[0] += stats.remoteSegmentsOverThreshold[0]
	a.remoteSegmentsOverThreshold[1] += stats.remoteSegmentsOverThreshold[1]
	a.remoteSegmentsOverThreshold[2] += stats.remoteSegmentsOverThreshold[2]
	a.remoteSegmentsOverThreshold[3] += stats.remoteSegmentsOverThreshold[3]
	a.remoteSegmentsOverThreshold[4] += stats.remoteSegmentsOverThreshold[4]
}

func getRSString(min, repair, success, total int) string {
	return fmt.Sprintf("%d/%d/%d/%d", min, repair, success, total)
}
