// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/accounting"
)

// Rollup is the service for totalling data on storage nodes for 1, 7, 30 day intervals
type Rollup interface {
	Run(ctx context.Context) error
}

type rollup struct {
	logger *zap.Logger
	ticker *time.Ticker
	db     accounting.DB
}

func newRollup(logger *zap.Logger, db accounting.DB, interval time.Duration) *rollup {
	return &rollup{
		logger: logger,
		ticker: time.NewTicker(interval),
		db:     db,
	}
}

// Run the rollup loop
func (r *rollup) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err = r.Query(ctx)
		if err != nil {
			r.logger.Error("Query failed", zap.Error(err))
		}

		select {
		case <-r.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the rollup is canceled via context
			return ctx.Err()
		}
	}
}

func (r *rollup) Query(ctx context.Context) error {
	//TODO
	// "Storage nodes ... will be paid for storing the data month-by-month.  At the end of the payment period, a Satellite
	// will calculate earnings for each of its storage nodes. Provided the storage node hasn’t been disqualified, the storage
	// node will be paid by the Satellite for the data it has stored over the course of the month, per the Satellite’s records"
	// see also https://github.com/storj/storj/blob/cb74d91cb07d659fd9b2fedb2629f23c8918ef0b/pkg/piecestore/psserver/store.go#L97

	// Payments Design Doc:
	// A rollup will provide a query that performs the following: Select each NodeID in the granular table
	// For each bucket (1 day, 7 day, 30 day): -- note: for alpha we're sticking to daily and monthly
	// Coalesce each row from the granular table by hour from 12:00 AM UTC to 11:00 PM UTC
	// if a row is missing between two hourly buckets, populate the value with the value from the previous hour.
	// Total the rows for the node and update into the respective bucket.

	lastRollup, isNil, err := r.db.LastRawTime(ctx, accounting.LastRollup)
	if err != nil {
		return Error.Wrap(err)
	}

	var tallies []string
	if isNil {
		r.logger.Info("Rollup found no existing raw tally data")
		tallies, err = r.db.GetRaw(ctx)
	} else {
		tallies, err = r.db.GetRawSince(ctx, lastRollup)
	}
	if err != nil {
		return Error.Wrap(err)
	}
	r.logger.Debug(fmt.Sprintf("%+v", tallies))

	return nil
}
