// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Config contains configurable values for rollup
type Config struct {
	Interval      time.Duration `help:"how frequently rollup should run" devDefault:"120s" default:"6h"`
	MaxAlphaUsage memory.Size   `help:"the bandwidth and storage usage limit for the alpha release" default:"25GB"`
}

// Service is the rollup service for totalling data on storage nodes on daily intervals
type Service struct {
	logger *zap.Logger
	ticker *time.Ticker
	db     accounting.DB
}

// New creates a new rollup service
func New(logger *zap.Logger, db accounting.DB, interval time.Duration) *Service {
	return &Service{
		logger: logger,
		ticker: time.NewTicker(interval),
		db:     db,
	}
}

// Run the Rollup loop
func (r *Service) Run(ctx context.Context) (err error) {
	r.logger.Info("Rollup service starting up")
	defer mon.Task()(&ctx)(&err)
	for {
		err = r.Rollup(ctx)
		if err != nil {
			r.logger.Error("Query failed", zap.Error(err))
		}
		select {
		case <-r.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the Rollup is canceled via context
			return ctx.Err()
		}
	}
}

// Rollup aggregates storage and bandwidth amounts for the time interval
func (r *Service) Rollup(ctx context.Context) error {
	// only Rollup new things - get LastRollup
	lastRollup, err := r.db.LastTimestamp(ctx, accounting.LastRollup)
	if err != nil {
		return Error.Wrap(err)
	}
	rollupStats := make(accounting.RollupStats)
	latestTally, err := r.RollupStorage(ctx, lastRollup, rollupStats)
	if err != nil {
		return Error.Wrap(err)
	}
	if len(rollupStats) == 0 {
		r.logger.Info("RollupStats is empty after RollupStorage")
	}
	err = r.RollupBW(ctx, lastRollup, rollupStats)
	if err != nil {
		return Error.Wrap(err)
	}
	if len(rollupStats) == 0 {
		r.logger.Info("RollupStats is empty after RollupBW")
		return nil
	}
	err = r.db.SaveRollup(ctx, latestTally, rollupStats)
	if err != nil {
		return Error.Wrap(err)
	}
	// Delete already rolled up tallies
	err = r.db.DeleteRawBefore(ctx, latestTally)
	if err != nil {
		return Error.Wrap(err)
	}
	return nil
}

// RollupStorage rolls up storage tally, modifies rollupStats map
func (r *Service) RollupStorage(ctx context.Context, lastRollup time.Time, rollupStats accounting.RollupStats) (latestTally time.Time, err error) {
	tallies, err := r.db.GetRawSince(ctx, lastRollup)
	if err != nil {
		return time.Now(), Error.Wrap(err)
	}
	if len(tallies) == 0 {
		r.logger.Info("Rollup found no new tallies")
		return time.Now(), nil
	}
	//loop through tallies and build Rollup
	for _, tallyRow := range tallies {
		node := tallyRow.NodeID
		if tallyRow.CreatedAt.After(latestTally) {
			latestTally = tallyRow.CreatedAt
		}
		//create or get AccoutingRollup day entry
		iDay := tallyRow.IntervalEndTime
		iDay = time.Date(iDay.Year(), iDay.Month(), iDay.Day(), 0, 0, 0, 0, iDay.Location())
		if rollupStats[iDay] == nil {
			rollupStats[iDay] = make(map[storj.NodeID]*accounting.Rollup)
		}
		if rollupStats[iDay][node] == nil {
			rollupStats[iDay][node] = &accounting.Rollup{NodeID: node, StartTime: iDay}
		}
		//increment data at rest sum
		switch tallyRow.DataType {
		case accounting.AtRest:
			rollupStats[iDay][node].AtRestTotal += tallyRow.DataTotal
		default:
			r.logger.Info("rollupStorage no longer supports non-accounting.AtRest datatypes")
		}
	}
	//remove the latest day (which we cannot know is complete), then push to DB
	latestTally = time.Date(latestTally.Year(), latestTally.Month(), latestTally.Day(), 0, 0, 0, 0, latestTally.Location())
	delete(rollupStats, latestTally)
	if len(rollupStats) == 0 {
		r.logger.Info("Rollup only found tallies for today")
		return time.Now(), nil
	}

	return latestTally, nil
}

// RollupBW aggregates the bandwidth rollups, modifies rollupStats map
func (r *Service) RollupBW(ctx context.Context, lastRollup time.Time, rollupStats accounting.RollupStats) error {
	var latestTally time.Time
	bws, err := r.db.GetStoragenodeBandwidthSince(ctx, lastRollup)
	if err != nil {
		return Error.Wrap(err)
	}
	if len(bws) == 0 {
		r.logger.Info("Rollup found no new bw rollups")
		return nil
	}
	for _, row := range bws {
		nodeID := row.NodeID
		if row.IntervalStart.After(latestTally) {
			latestTally = row.IntervalStart
		}
		day := row.IntervalStart
		day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
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
	//remove the latest day (which we cannot know is complete), then push to DB
	latestTally = time.Date(latestTally.Year(), latestTally.Month(), latestTally.Day(), 0, 0, 0, 0, latestTally.Location())
	delete(rollupStats, latestTally)
	if len(rollupStats) == 0 {
		r.logger.Info("Rollup only found data for today")
	}
	return nil
}
