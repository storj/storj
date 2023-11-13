// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"fmt"
	"sync"

	"github.com/spacemonkeygo/monkit/v3"
)

// statsCollector holds a *stats for each redundancy scheme
// seen by the repairer. These are chained into the monkit scope for
// monitoring as they are initialized.
type statsCollector struct {
	lock  sync.Mutex
	stats map[string]*stats
}

func newStatsCollector() *statsCollector {
	return &statsCollector{
		stats: make(map[string]*stats),
	}
}

func (collector *statsCollector) getStatsByRS(rs string) *stats {
	collector.lock.Lock()
	defer collector.lock.Unlock()

	stats, ok := collector.stats[rs]
	if !ok {
		stats = newStats(rs)
		mon.Chain(stats)
		collector.stats[rs] = stats
	}
	return stats
}

// stats is used for collecting and reporting repairer metrics.
//
// add any new metrics tagged with rs_scheme to this struct and set them
// in newStats.
type stats struct {
	repairAttempts              *monkit.Meter
	repairSegmentSize           *monkit.IntVal
	repairerSegmentsBelowMinReq *monkit.Counter
	repairerNodesUnavailable    *monkit.Meter
	repairUnnecessary           *monkit.Meter
	healthyRatioBeforeRepair    *monkit.FloatVal
	repairTooManyNodesFailed    *monkit.Meter
	repairFailed                *monkit.Meter
	repairPartial               *monkit.Meter
	repairSuccess               *monkit.Meter
	healthyRatioAfterRepair     *monkit.FloatVal
	segmentTimeUntilRepair      *monkit.IntVal
	segmentRepairCount          *monkit.IntVal
}

func newStats(rs string) *stats {
	return &stats{
		repairAttempts:              monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "repair_attempts").WithTag("rs_scheme", rs)),
		repairSegmentSize:           monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "repair_segment_size").WithTag("rs_scheme", rs)),
		repairerSegmentsBelowMinReq: monkit.NewCounter(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "repairer_segments_below_min_req").WithTag("rs_scheme", rs)),
		repairerNodesUnavailable:    monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "repairer_nodes_unavailable").WithTag("rs_scheme", rs)),
		repairUnnecessary:           monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "repair_unnecessary").WithTag("rs_scheme", rs)),
		healthyRatioBeforeRepair:    monkit.NewFloatVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "healthy_ratio_before_repair").WithTag("rs_scheme", rs)),
		repairTooManyNodesFailed:    monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "repair_too_many_nodes_failed").WithTag("rs_scheme", rs)),
		repairFailed:                monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "repair_failed").WithTag("rs_scheme", rs)),
		repairPartial:               monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "repair_partial").WithTag("rs_scheme", rs)),
		repairSuccess:               monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "repair_success").WithTag("rs_scheme", rs)),
		healthyRatioAfterRepair:     monkit.NewFloatVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "healthy_ratio_after_repair").WithTag("rs_scheme", rs)),
		segmentTimeUntilRepair:      monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "segment_time_until_repair").WithTag("rs_scheme", rs)),
		segmentRepairCount:          monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").WithTag("name", "segment_repair_count").WithTag("rs_scheme", rs)),
	}
}

// Stats implements the monkit.StatSource interface.
func (stats *stats) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	stats.repairAttempts.Stats(cb)
	stats.repairSegmentSize.Stats(cb)
	stats.repairerSegmentsBelowMinReq.Stats(cb)
	stats.repairerNodesUnavailable.Stats(cb)
	stats.repairUnnecessary.Stats(cb)
	stats.healthyRatioBeforeRepair.Stats(cb)
	stats.repairTooManyNodesFailed.Stats(cb)
	stats.repairFailed.Stats(cb)
	stats.repairPartial.Stats(cb)
	stats.repairSuccess.Stats(cb)
	stats.healthyRatioAfterRepair.Stats(cb)
	stats.segmentTimeUntilRepair.Stats(cb)
	stats.segmentRepairCount.Stats(cb)
}

func getRSString(min, repair, success, total int) string {
	return fmt.Sprintf("%d/%d/%d/%d", min, repair, success, total)
}
