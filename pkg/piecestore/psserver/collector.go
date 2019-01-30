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

// ErrorCollector is error class for piece collector
var ErrorCollector = errs.Class("piecestore collector")

// Collector collects expired pieces from database and disk.
type Collector struct {
	log     *zap.Logger
	db      *psdb.DB
	storage *pstore.Storage

	interval time.Duration
}

// NewCollector returns a new piece collector
func NewCollector(log *zap.Logger, db *psdb.DB, storage *pstore.Storage, interval time.Duration) *Collector {
	return &Collector{
		log:      log,
		db:       db,
		storage:  storage,
		interval: interval,
	}
}

// Run runs the collector at regular intervals
func (service *Collector) Run(ctx context.Context) error {
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

// Collect collects expired pieces att this moment.
func (service *Collector) Collect(ctx context.Context) error {
	for {
		expired, err := service.db.DeleteExpired(ctx)
		if err != nil {
			return ErrorCollector.Wrap(err)
		}
		if len(expired) == 0 {
			return nil
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
