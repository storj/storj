// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup

import (
	"context"
	"time"

	"go.uber.org/zap"

	dbx "storj.io/storj/pkg/accounting/dbx"
)

// Rollup is the service for totalling data on storage nodes for 1, 7, 30 day intervals
type Rollup interface {
	Run(ctx context.Context) error
}

type rollup struct {
	logger *zap.Logger
	ticker *time.Ticker
	db     *accountingdb.Database
}

func newRollup(logger *zap.Logger, db *accountingdb.Database, interval time.Duration) (*rollup, error) {
	return &rollup{
		logger: logger,
		ticker: time.NewTicker(interval),
		db:     db,
	}, nil
}

// Run the rollup loop
func (r *rollup) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err = r.Query(ctx)
		if err != nil {
			Error.Wrap(err)
		}

		select {
		case <-r.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the rollup is canceled via context
			err = r.db.Close()
			if err != nil {
				Error.Wrap(err)
			}
			return ctx.Err()
		}
	}
}

func (r *rollup) Query(ctx context.Context) error {
	//TODO
	return nil
}
