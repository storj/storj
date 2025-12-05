// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package bandwidth implements bandwidth usage rollup loop.
package bandwidth

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/sync2"
)

var mon = monkit.Package()

// Config defines parameters for storage node Collector.
type Config struct {
	Interval time.Duration `help:"how frequently bandwidth usage cache should be synced with the db" default:"1h0m0s" testDefault:"1s"`
}

// Service implements the bandwidth usage rollup service.
//
// architecture: Chore
type Service struct {
	log   *zap.Logger
	cache *Cache
	Loop  *sync2.Cycle
}

// NewService creates a new bandwidth service.
func NewService(log *zap.Logger, cache *Cache, config Config) *Service {
	return &Service{
		log:   log,
		cache: cache,
		Loop:  sync2.NewCycle(config.Interval),
	}
}

// Run starts the background process for syncing bandwidth usage cache with the db.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.Loop.Run(ctx, service.RunOnce)
}

// RunOnce syncs bandwidth usage cache with the db.
func (service *Service) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	service.log.Info("Persisting bandwidth usage cache to db")
	err = service.cache.Persist(ctx)
	if err != nil {
		service.log.Error("Could not persist bandwidth cache to db", zap.Error(err))
	}
	return nil
}

// Close stops the background process for rollups of bandwidth usage.
func (service *Service) Close() (err error) {
	service.Loop.Close()
	return nil
}
