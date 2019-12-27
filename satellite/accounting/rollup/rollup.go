// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/accounting"
)

// Config contains configurable values for rollup
type Config struct {
	Interval      time.Duration `help:"how frequently rollup should run" releaseDefault:"24h" devDefault:"120s"`
	MaxAlphaUsage memory.Size   `help:"the bandwidth and storage usage limit for the alpha release" default:"25GB"`
	DeleteTallies bool          `help:"option for deleting tallies after they are rolled up" default:"true"`
}

// Service is the rollup service for totalling data on storage nodes on daily intervals
//
// architecture: Chore
type Service struct {
	logger        *zap.Logger
	Loop          sync2.Cycle
	sdb           accounting.StoragenodeAccounting
	deleteTallies bool
}

// New creates a new rollup service
func New(logger *zap.Logger, sdb accounting.StoragenodeAccounting, interval time.Duration, deleteTallies bool) *Service {
	return &Service{
		logger:        logger,
		Loop:          *sync2.NewCycle(interval),
		sdb:           sdb,
		deleteTallies: deleteTallies,
	}
}

// Run the Rollup loop
func (r *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	r.logger.Info("Rollup service starting up")
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

// Rollup aggregates storage and bandwidth amounts for the time interval
func (r *Service) Rollup(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	// only Rollup new things - get LastRollup
	lastRollup, err := r.sdb.LastTimestamp(ctx, accounting.LastRollup)
	if err != nil {
		return Error.Wrap(err)
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

	//remove the latest day (which we cannot know is complete), then push to DB
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

	if r.deleteTallies {
		// Delete already rolled up tallies
		err = r.sdb.DeleteTalliesBefore(ctx, latestTally)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// RollupStorage rolls up storage tally, modifies rollupStats map
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
	//loop through tallies and build Rollup
	for _, tallyRow := range tallies {
		node := tallyRow.NodeID
		// tallyEndTime is the time the at rest tally was saved
		tallyEndTime := tallyRow.IntervalEndTime.UTC()
		if tallyEndTime.After(latestTally) {
			latestTally = tallyEndTime
		}
		//create or get AccoutingRollup day entry
		iDay := time.Date(tallyEndTime.Year(), tallyEndTime.Month(), tallyEndTime.Day(), 0, 0, 0, 0, tallyEndTime.Location())
		if rollupStats[iDay] == nil {
			rollupStats[iDay] = make(map[storj.NodeID]*accounting.Rollup)
		}
		if rollupStats[iDay][node] == nil {
			rollupStats[iDay][node] = &accounting.Rollup{NodeID: node, StartTime: iDay}
		}
		//increment data at rest sum
		rollupStats[iDay][node].AtRestTotal += tallyRow.DataTotal
	}

	return latestTally, nil
}

// RollupBW aggregates the bandwidth rollups, modifies rollupStats map
func (r *Service) RollupBW(ctx context.Context, lastRollup time.Time, rollupStats accounting.RollupStats) (err error) {
	defer mon.Task()(&ctx)(&err)
	var latestTally time.Time
	bws, err := r.sdb.GetBandwidthSince(ctx, lastRollup.UTC())
	if err != nil {
		return Error.Wrap(err)
	}
	if len(bws) == 0 {
		r.logger.Info("Rollup found no new bw rollups")
		return nil
	}
	for _, row := range bws {
		nodeID := row.NodeID
		// interval is the time the bw order was saved
		interval := row.IntervalStart.UTC()
		if interval.After(latestTally) {
			latestTally = interval
		}
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
	}

	return nil
}
