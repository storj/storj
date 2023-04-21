// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"fmt"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/uuid"
)

// statsCollector holds a *stats for each redundancy scheme
// seen by the checker. These are chained into the monkit scope for
// monitoring as they are initialized.
type statsCollector struct {
	stats map[string]*stats
}

func newStatsCollector() *statsCollector {
	return &statsCollector{
		stats: make(map[string]*stats),
	}
}

func (collector *statsCollector) getStatsByRS(rs string) *stats {
	stats, ok := collector.stats[rs]
	if !ok {
		stats = newStats(rs)
		mon.Chain(stats)
		collector.stats[rs] = stats
	}
	return stats
}

// collectAggregates transfers the iteration aggregates into the
// respective stats monkit metrics at the end of each checker iteration.
// iterationAggregates is then cleared.
func (collector *statsCollector) collectAggregates() {
	for _, stats := range collector.stats {
		stats.collectAggregates()
		stats.iterationAggregates = new(aggregateStats)
	}
}

// stats is used for collecting and reporting checker metrics.
//
// add any new metrics tagged with rs_scheme to this struct and set them
// in newStats.
type stats struct {
	iterationAggregates *aggregateStats

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

	segmentsBelowMinReq         *monkit.Counter
	segmentTotalCount           *monkit.IntVal
	segmentHealthyCount         *monkit.IntVal
	segmentClumpedCount         *monkit.IntVal
	segmentAge                  *monkit.IntVal
	segmentHealth               *monkit.FloatVal
	injuredSegmentHealth        *monkit.FloatVal
	segmentTimeUntilIrreparable *monkit.IntVal
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

func newStats(rs string) *stats {
	return &stats{
		iterationAggregates:             new(aggregateStats),
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
		segmentsBelowMinReq:             monkit.NewCounter(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segments_below_min_req").WithTag("rs_scheme", rs)),
		segmentTotalCount:               monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_total_count").WithTag("rs_scheme", rs)),
		segmentHealthyCount:             monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_healthy_count").WithTag("rs_scheme", rs)),
		segmentClumpedCount:             monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_clumped_count").WithTag("rs_scheme", rs)),
		segmentAge:                      monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_age").WithTag("rs_scheme", rs)),
		segmentHealth:                   monkit.NewFloatVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_health").WithTag("rs_scheme", rs)),
		injuredSegmentHealth:            monkit.NewFloatVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_injured_segment_health").WithTag("rs_scheme", rs)),
		segmentTimeUntilIrreparable:     monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "checker_segment_time_until_irreparable").WithTag("rs_scheme", rs)),
	}
}

func (stats *stats) collectAggregates() {
	stats.objectsChecked.Observe(stats.iterationAggregates.objectsChecked)
	stats.remoteSegmentsChecked.Observe(stats.iterationAggregates.remoteSegmentsChecked)
	stats.remoteSegmentsNeedingRepair.Observe(stats.iterationAggregates.remoteSegmentsNeedingRepair)
	stats.newRemoteSegmentsNeedingRepair.Observe(stats.iterationAggregates.newRemoteSegmentsNeedingRepair)
	stats.remoteSegmentsLost.Observe(stats.iterationAggregates.remoteSegmentsLost)
	stats.objectsLost.Observe(int64(len(stats.iterationAggregates.objectsLost)))
	stats.remoteSegmentsFailedToCheck.Observe(stats.iterationAggregates.remoteSegmentsFailedToCheck)
	stats.remoteSegmentsOverThreshold1.Observe(stats.iterationAggregates.remoteSegmentsOverThreshold[0])
	stats.remoteSegmentsOverThreshold2.Observe(stats.iterationAggregates.remoteSegmentsOverThreshold[1])
	stats.remoteSegmentsOverThreshold3.Observe(stats.iterationAggregates.remoteSegmentsOverThreshold[2])
	stats.remoteSegmentsOverThreshold4.Observe(stats.iterationAggregates.remoteSegmentsOverThreshold[3])
	stats.remoteSegmentsOverThreshold5.Observe(stats.iterationAggregates.remoteSegmentsOverThreshold[4])

	allUnhealthy := stats.iterationAggregates.remoteSegmentsNeedingRepair + stats.iterationAggregates.remoteSegmentsFailedToCheck
	allChecked := stats.iterationAggregates.remoteSegmentsChecked
	allHealthy := allChecked - allUnhealthy

	stats.remoteSegmentsHealthyPercentage.Observe(100 * float64(allHealthy) / float64(allChecked))
}

// Stats implements the monkit.StatSource interface.
func (stats *stats) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	stats.objectsChecked.Stats(cb)
	stats.remoteSegmentsChecked.Stats(cb)
	stats.remoteSegmentsNeedingRepair.Stats(cb)
	stats.newRemoteSegmentsNeedingRepair.Stats(cb)
	stats.remoteSegmentsLost.Stats(cb)
	stats.objectsLost.Stats(cb)
	stats.remoteSegmentsFailedToCheck.Stats(cb)
	stats.remoteSegmentsOverThreshold1.Stats(cb)
	stats.remoteSegmentsOverThreshold2.Stats(cb)
	stats.remoteSegmentsOverThreshold3.Stats(cb)
	stats.remoteSegmentsOverThreshold4.Stats(cb)
	stats.remoteSegmentsOverThreshold5.Stats(cb)
	stats.remoteSegmentsHealthyPercentage.Stats(cb)
	stats.segmentsBelowMinReq.Stats(cb)
	stats.segmentTotalCount.Stats(cb)
	stats.segmentHealthyCount.Stats(cb)
	stats.segmentAge.Stats(cb)
	stats.segmentHealth.Stats(cb)
	stats.injuredSegmentHealth.Stats(cb)
	stats.segmentTimeUntilIrreparable.Stats(cb)
}

func getRSString(min, repair, success, total int) string {
	return fmt.Sprintf("%d/%d/%d/%d", min, repair, success, total)
}
