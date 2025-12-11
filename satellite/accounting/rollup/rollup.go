// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/eventkit"
	"storj.io/storj/satellite/accounting"
)

// Config contains configurable values for rollup.
type Config struct {
	Interval                time.Duration `help:"how frequently rollup should run" releaseDefault:"24h" devDefault:"120s" testDefault:"$TESTINTERVAL"`
	DeleteTallies           bool          `help:"option for deleting tallies after they are rolled up" default:"true"`
	DeleteTalliesBatchSize  int           `help:"how many tallies to delete in a batch" default:"10000"`
	EventkitTrackingEnabled bool          `help:"whether to emit eventkit events for storage and bandwidth rollup" default:"false"`
}

// Service is the rollup service for totalling data on storage nodes on daily intervals.
//
// architecture: Chore
type Service struct {
	logger                  *zap.Logger
	Loop                    *sync2.Cycle
	sdb                     accounting.StoragenodeAccounting
	deleteTallies           bool
	deleteTalliesBatchSize  int
	OrderExpiration         time.Duration
	eventkitTrackingEnabled bool
}

// New creates a new rollup service.
func New(logger *zap.Logger, sdb accounting.StoragenodeAccounting, config Config, orderExpiration time.Duration) *Service {
	return &Service{
		logger:                  logger,
		Loop:                    sync2.NewCycle(config.Interval),
		sdb:                     sdb,
		deleteTallies:           config.DeleteTallies,
		deleteTalliesBatchSize:  config.DeleteTalliesBatchSize,
		OrderExpiration:         orderExpiration,
		eventkitTrackingEnabled: config.EventkitTrackingEnabled,
	}
}

// Run the Rollup loop.
func (r *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return r.Loop.Run(ctx, func(ctx context.Context) error {
		err := r.Rollup(ctx)
		if err != nil {
			r.logger.Error("rollup failed", zap.Error(err))
		}
		return nil
	})
}

// Close stops the service and releases any resources.
func (r *Service) Close() error {
	r.Loop.Close()
	return nil
}

// Rollup aggregates storage and bandwidth amounts for the time interval.
func (r *Service) Rollup(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	// only Rollup new things - get LastRollup
	lastRollup, err := r.sdb.LastTimestamp(ctx, accounting.LastRollup)
	if err != nil {
		return Error.Wrap(err)
	}
	// unexpired orders with created at times before the last rollup timestamp could still have been added later
	if !lastRollup.IsZero() {
		lastRollup = lastRollup.Add(-r.OrderExpiration)
	}

	rollupStats := make(accounting.RollupStats)
	latestTally, err := r.RollupStorage(ctx, lastRollup, rollupStats)
	if err != nil {
		return Error.Wrap(err)
	}

	err = r.RollupBW(ctx, lastRollup, rollupStats)
	if err != nil {
		return Error.Wrap(err)
	}

	// remove the latest day (which we cannot know is complete), then push to DB
	latestTally = time.Date(latestTally.Year(), latestTally.Month(), latestTally.Day(), 0, 0, 0, 0, latestTally.Location())
	delete(rollupStats, latestTally)
	if len(rollupStats) == 0 {
		r.logger.Info("RollupStats is empty")
		return nil
	}

	err = r.sdb.SaveRollup(ctx, latestTally, rollupStats)
	if err != nil {
		return Error.Wrap(err)
	}

	// Emit eventkit events for storage and bandwidth rollup if enabled
	if r.eventkitTrackingEnabled {
		r.emitRollupEvents(rollupStats)
	}

	if r.deleteTallies {
		// Delete already rolled up tallies
		latestTally = latestTally.Add(-r.OrderExpiration)
		err = r.sdb.DeleteTalliesBefore(ctx, latestTally, r.deleteTalliesBatchSize)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// RollupStorage rolls up storage tally, modifies rollupStats map.
func (r *Service) RollupStorage(ctx context.Context, lastRollup time.Time, rollupStats accounting.RollupStats) (latestTally time.Time, err error) {
	defer mon.Task()(&ctx)(&err)
	tallies, err := r.sdb.GetTalliesSince(ctx, lastRollup)
	if err != nil {
		return lastRollup, Error.Wrap(err)
	}
	if len(tallies) == 0 {
		r.logger.Info("Rollup found no new tallies")
		return lastRollup, nil
	}
	// loop through tallies and build Rollup
	for _, tallyRow := range tallies {
		node := tallyRow.NodeID
		// tallyEndTime is the time the at rest tally was saved
		tallyEndTime := tallyRow.IntervalEndTime.UTC()
		if tallyEndTime.After(latestTally) {
			latestTally = tallyEndTime
		}
		// create or get AccoutingRollup day entry
		iDay := time.Date(tallyEndTime.Year(), tallyEndTime.Month(), tallyEndTime.Day(), 0, 0, 0, 0, tallyEndTime.Location())
		if rollupStats[iDay] == nil {
			rollupStats[iDay] = make(map[storj.NodeID]*accounting.Rollup)
		}
		if rollupStats[iDay][node] == nil {
			rollupStats[iDay][node] = &accounting.Rollup{NodeID: node, StartTime: iDay}
		}
		// increment data at rest sum
		rollupStats[iDay][node].AtRestTotal += tallyRow.DataTotal

		// update interval_end_time to the latest tally end time for the day
		if rollupStats[iDay][node].IntervalEndTime.Before(tallyEndTime) {
			rollupStats[iDay][node].IntervalEndTime = tallyEndTime
		}
	}

	return latestTally, nil
}

// RollupBW aggregates the bandwidth rollups, modifies rollupStats map.
func (r *Service) RollupBW(ctx context.Context, lastRollup time.Time, rollupStats accounting.RollupStats) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = r.sdb.GetBandwidthSince(ctx, lastRollup.UTC(), func(ctx context.Context, row *accounting.StoragenodeBandwidthRollup) error {
		nodeID := row.NodeID
		// interval is the time the bw order was saved
		interval := row.IntervalStart.UTC()
		day := time.Date(interval.Year(), interval.Month(), interval.Day(), 0, 0, 0, 0, interval.Location())
		if rollupStats[day] == nil {
			rollupStats[day] = make(map[storj.NodeID]*accounting.Rollup)
		}
		if rollupStats[day][nodeID] == nil {
			rollupStats[day][nodeID] = &accounting.Rollup{NodeID: nodeID, StartTime: day}
		}
		switch row.Action {
		case uint(pb.PieceAction_INVALID):
			r.logger.Info("invalid order action type")
		case uint(pb.PieceAction_PUT):
			rollupStats[day][nodeID].PutTotal += int64(row.Settled)
		case uint(pb.PieceAction_GET):
			rollupStats[day][nodeID].GetTotal += int64(row.Settled)
		case uint(pb.PieceAction_GET_AUDIT):
			rollupStats[day][nodeID].GetAuditTotal += int64(row.Settled)
		case uint(pb.PieceAction_GET_REPAIR):
			rollupStats[day][nodeID].GetRepairTotal += int64(row.Settled)
		case uint(pb.PieceAction_PUT_REPAIR):
			rollupStats[day][nodeID].PutRepairTotal += int64(row.Settled)
		default:
			r.logger.Info("delete order type")
		}

		return nil
	})
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// emitRollupEvents emits eventkit events for storage and bandwidth rollup data.
func (r *Service) emitRollupEvents(rollupStats accounting.RollupStats) {
	for day, nodeRollups := range rollupStats {
		for nodeID, rollup := range nodeRollups {
			// Emit storage rollup event if there's storage data
			if rollup.AtRestTotal > 0 {
				ek.Event("storage_rollup",
					eventkit.Bytes("node_id", nodeID.Bytes()),
					eventkit.String("tenant_id", ""), // Reserved for future use
					eventkit.Timestamp("day", day),
					eventkit.Timestamp("interval_start", rollup.StartTime),
					eventkit.Timestamp("interval_end", rollup.IntervalEndTime),
					eventkit.Float64("at_rest_total", rollup.AtRestTotal),
					eventkit.String("event_type", "time_aggregated"),
				)
			}

			// Emit bandwidth rollup event if there's bandwidth data
			hasBandwidth := rollup.PutTotal > 0 || rollup.GetTotal > 0 ||
				rollup.GetAuditTotal > 0 || rollup.GetRepairTotal > 0 || rollup.PutRepairTotal > 0
			if hasBandwidth {
				ek.Event("bandwidth_rollup",
					eventkit.Bytes("node_id", nodeID.Bytes()),
					eventkit.String("tenant_id", ""), // Reserved for future use
					eventkit.Timestamp("day", day),
					eventkit.Int64("put_total", rollup.PutTotal),
					eventkit.Int64("get_total", rollup.GetTotal),
					eventkit.Int64("get_audit_total", rollup.GetAuditTotal),
					eventkit.Int64("get_repair_total", rollup.GetRepairTotal),
					eventkit.Int64("put_repair_total", rollup.PutRepairTotal),
					eventkit.String("event_type", "time_aggregated"),
				)
			}
		}
	}
}
