// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/storj"
)

// statsCollector holds a *stats for each redundancy scheme and placement seen by reparier. These
// are chained into the monkit scope for monitoring as they are initialized.
type statsCollector struct {
	lock  sync.Mutex
	stats map[string]*stats
}

func newStatsCollector() *statsCollector {
	return &statsCollector{
		stats: make(map[string]*stats),
	}
}

func (collector *statsCollector) getStats(rsSchema string, placement string) *stats {
	collector.lock.Lock()
	defer collector.lock.Unlock()

	key := fmt.Sprintf("%s-%s", rsSchema, placement)

	stats, ok := collector.stats[key]
	if !ok {
		stats = newStats(rsSchema, placement)
		mon.Chain(stats)
		collector.stats[key] = stats
	}
	return stats
}

// stats is used for collecting and reporting repairer metrics for a specific RS Schema and
// placement.
//
// Add any new metrics tagged with rs_scheme, and placement to this struct and set them in newStats.
type stats struct {
	repairAttempts                        *monkit.Meter
	repairSegmentSize                     *monkit.IntVal
	repairerSegmentsBelowMinReq           *monkit.Counter
	repairerNodesUnavailable              *monkit.Meter
	repairUnnecessary                     *monkit.Meter
	healthyRatioBeforeRepair              *monkit.FloatVal
	repairTooManyNodesFailed              *monkit.Meter
	repairFailed                          *monkit.Meter
	repairPartial                         *monkit.Meter
	repairSuccess                         *monkit.Meter
	healthyRatioAfterRepair               *monkit.FloatVal
	segmentTimeUntilRepair                *monkit.IntVal
	segmentRepairCount                    *monkit.IntVal
	segmentExpiredBeforeRepair            *monkit.Meter
	droppedUndesirablePiecesWithoutRepiar *monkit.Meter
	repairerUnnecessaryDownloads          *monkit.Counter
	repairerRequiredDownloads             *monkit.Counter
	repairSuspectedNetworkProblem         *monkit.Meter
	repairBytesUploaded                   *monkit.Meter
}

func newStats(rsSchema string, placement string) *stats {
	return &stats{
		repairAttempts: monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repair_attempts").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		repairSegmentSize: monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repair_segment_size").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		repairerSegmentsBelowMinReq: monkit.NewCounter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repairer_segments_below_min_req").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		repairerNodesUnavailable: monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repairer_nodes_unavailable").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		repairUnnecessary: monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repair_unnecessary").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		healthyRatioBeforeRepair: monkit.NewFloatVal(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "healthy_ratio_before_repair").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		repairTooManyNodesFailed: monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repair_too_many_nodes_failed").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		repairFailed: monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repair_failed").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		repairPartial: monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repair_partial").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		repairSuccess: monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repair_success").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		healthyRatioAfterRepair: monkit.NewFloatVal(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "healthy_ratio_after_repair").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		segmentTimeUntilRepair: monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "segment_time_until_repair").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		segmentRepairCount: monkit.NewIntVal(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "segment_repair_count").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		segmentExpiredBeforeRepair: monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "segment_expired_before_repair").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		droppedUndesirablePiecesWithoutRepiar: monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "dropped_undesirable_pieces_without_repair").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		repairerUnnecessaryDownloads: monkit.NewCounter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repairer_unnecessary_downloads").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		repairerRequiredDownloads: monkit.NewCounter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repairer_required_downloads").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		repairSuspectedNetworkProblem: monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repair_suspected_network_problem").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
		repairBytesUploaded: monkit.NewMeter(monkit.NewSeriesKey("tagged_repair_stats").
			WithTag("name", "repair_bytes_uploaded").WithTag("rs_scheme", rsSchema).WithTag("placement", placement)),
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

func getRSString(rs storj.RedundancyScheme) string {
	return fmt.Sprintf("%d/%d/%d/%d", rs.RequiredShares, rs.RepairShares, rs.OptimalShares, rs.TotalShares)
}

func getPlacementString(p storj.PlacementConstraint) string {
	return strconv.FormatUint(uint64(p), 10)
}
