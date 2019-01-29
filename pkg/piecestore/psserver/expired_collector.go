// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
)

var ErrorCollector = errs.Class("piecestore collector")

// ExpiredCollector collects expired pieces from database and disk.
type ExpiredCollector struct {
	log     *zap.Logger
	db      *psdb.DB
	storage *pstore.Storage

	interval time.Duration
}

// NewExpiredCollector returns a new expired collector
func NewExpiredCollector(log *zap.Logger, db *psdb.DB, storage *pstore.Storage, interval time.Duration) *ExpiredCollector {
	return &ExpiredCollector{
		log:      log,
		db:       db,
		storage:  storage,
		interval: interval,
	}
}

// Run runs the collector at regular intervals
func (service *ExpiredCollector) Run(ctx context.Context) error {
	ticker := time.NewTicker(service.interval)
	defer ticker.Stop()

	for {
		err := service.Collect(ctx)
		if err != nil {
			service.log.Error("collect", zap.Error(err))
		}

		select {
		case <-ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the bucket refresher service is canceled via context
			return ctx.Err()
		}
	}
}

// Collects runs a single collect
func (service *ExpiredCollector) Collect(ctx context.Context) error {
	for {
		expired, err := service.db.DeleteExpired(ctx)
		if len(expired) == 0 {
			return nil
		}
		if err != nil {
			return ErrorCollector.Wrap(err)
		}

		var errlist errs.Group
		for _, id := range expired {
			errlist.Add(service.storage.Delete(id))
		}

		if err := errlist.Err(); err != nil {
			return ErrorCollector.Wrap(err)
		}
	}
}
