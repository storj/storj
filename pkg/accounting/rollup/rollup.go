// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/storj"
)

// Config contains configurable values for rollup
type Config struct {
	Interval time.Duration `help:"how frequently rollup should run" default:"120s"`
}

// Rollup is the service for totalling data on storage nodes on daily intervals
type Rollup struct { // TODO: rename to service
	logger *zap.Logger
	ticker *time.Ticker
	db     accounting.DB
}

// New creates a new rollup service
func New(logger *zap.Logger, db accounting.DB, interval time.Duration) *Rollup {
	return &Rollup{
		logger: logger,
		ticker: time.NewTicker(interval),
		db:     db,
	}
}

// Run the Rollup loop
func (r *Rollup) Run(ctx context.Context) (err error) {
	r.logger.Info("Rollup service starting up")
	defer mon.Task()(&ctx)(&err)
	for {
		err = r.Query(ctx)
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

// Query rolls up raw tally
func (r *Rollup) Query(ctx context.Context) error {
	// only Rollup new things - get LastRollup
	var latestTally time.Time
	lastRollup, err := r.db.LastTimestamp(ctx, accounting.LastRollup)
	if err != nil {
		return Error.Wrap(err)
	}
	tallies, err := r.db.GetRawSince(ctx, lastRollup)
	if err != nil {
		return Error.Wrap(err)
	}
	if len(tallies) == 0 {
		r.logger.Info("Rollup found no new tallies")
		return nil
	}
	//loop through tallies and build Rollup
	rollupStats := make(accounting.RollupStats)
	for _, tallyRow := range tallies {
		node := tallyRow.NodeID
		if tallyRow.CreatedAt.After(latestTally) {
			latestTally = tallyRow.CreatedAt
		}
		//create or get AccoutingRollup
		iDay := tallyRow.IntervalEndTime
		iDay = time.Date(iDay.Year(), iDay.Month(), iDay.Day(), 0, 0, 0, 0, iDay.Location())
		if rollupStats[iDay] == nil {
			rollupStats[iDay] = make(map[storj.NodeID]*accounting.Rollup)
		}
		if rollupStats[iDay][node] == nil {
			rollupStats[iDay][node] = &accounting.Rollup{NodeID: node, StartTime: iDay}
		}
		//increment Rollups
		switch tallyRow.DataType {
		case accounting.BandwidthPut:
			rollupStats[iDay][node].PutTotal += int64(tallyRow.DataTotal)
		case accounting.BandwidthGet:
			rollupStats[iDay][node].GetTotal += int64(tallyRow.DataTotal)
		case accounting.BandwidthGetAudit:
			rollupStats[iDay][node].GetAuditTotal += int64(tallyRow.DataTotal)
		case accounting.BandwidthGetRepair:
			rollupStats[iDay][node].GetRepairTotal += int64(tallyRow.DataTotal)
		case accounting.BandwidthPutRepair:
			rollupStats[iDay][node].PutRepairTotal += int64(tallyRow.DataTotal)
		case accounting.AtRest:
			rollupStats[iDay][node].AtRestTotal += tallyRow.DataTotal
		default:
			return Error.Wrap(fmt.Errorf("Bad tally datatype in Rollup : %d", tallyRow.DataType))
		}
	}
	//remove the latest day (which we cannot know is complete), then push to DB
	latestTally = time.Date(latestTally.Year(), latestTally.Month(), latestTally.Day(), 0, 0, 0, 0, latestTally.Location())
	delete(rollupStats, latestTally)
	if len(rollupStats) == 0 {
		r.logger.Info("Rollup only found tallies for today")
		return nil
	}
	return Error.Wrap(r.db.SaveRollup(ctx, latestTally, rollupStats))
}
