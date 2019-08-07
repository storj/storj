// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"
	"math/rand"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/date"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
)

var (
	// NodeStatsCacheErr defines node stats cache loop error
	NodeStatsCacheErr = errs.Class("node stats cache error")
)

// Config defines nodestats cache configuration
type Config struct {
	MaxSleepDuration int           `help:"max number of seconds cache waits before performing each sync" releaseDefault:"300" devDefault:"1"`
	ReputationSync   time.Duration `help:"how often to sync reputation" releaseDefault:"4h" devDefault:"1m"`
	StorageSync      time.Duration `help:"how often to sync storage" releaseDefault:"12h" devDefault:"2m"`
}

// CacheStorage encapsulates cache DBs
type CacheStorage struct {
	Reputation   reputation.DB
	StorageUsage storageusage.DB
}

// Cache runs cache loop and stores reputation stats
// and storage usage into db
type Cache struct {
	log *zap.Logger

	db      CacheStorage
	service *Service
	trust   *trust.Pool

	maxSleepDuration int
	reputationCycle  *sync2.Cycle
	storageCycle     *sync2.Cycle
}

// NewCache creates new caching service instance
func NewCache(log *zap.Logger, config Config, db CacheStorage, service *Service, trust *trust.Pool) *Cache {
	return &Cache{
		log:              log,
		db:               db,
		service:          service,
		trust:            trust,
		maxSleepDuration: config.MaxSleepDuration,
		reputationCycle:  sync2.NewCycle(config.ReputationSync),
		storageCycle:     sync2.NewCycle(config.StorageSync),
	}
}

// Run runs loop
func (cache *Cache) Run(ctx context.Context) error {
	var group errgroup.Group

	cache.reputationCycle.Start(ctx, &group, func(ctx context.Context) error {
		sync2.Sleep(ctx, time.Duration(rand.Intn(cache.maxSleepDuration))*time.Second)

		err := cache.CacheReputationStats(ctx)
		if err != nil {
			cache.log.Error("Get stats query failed", zap.Error(err))
		}

		return nil
	})
	cache.storageCycle.Start(ctx, &group, func(ctx context.Context) error {
		sync2.Sleep(ctx, time.Duration(rand.Intn(cache.maxSleepDuration))*time.Second)

		err := cache.CacheSpaceUsage(ctx)
		if err != nil {
			cache.log.Error("Get disk space usage query failed", zap.Error(err))
		}

		return nil
	})

	return group.Wait()
}

// CacheReputationStats queries node stats from all the satellites
// known to the storagenode and stores information into db
func (cache *Cache) CacheReputationStats(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var cacheStatsErr errs.Group
	for _, satellite := range cache.trust.GetSatellites(ctx) {
		stats, err := cache.service.GetReputationStats(ctx, satellite)
		if err != nil {
			cacheStatsErr.Add(NodeStatsCacheErr.Wrap(err))
			continue
		}

		if err = cache.db.Reputation.Store(ctx, *stats); err != nil {
			cacheStatsErr.Add(NodeStatsCacheErr.Wrap(err))
			continue
		}
	}

	return cacheStatsErr.Err()
}

// CacheSpaceUsage queries disk space usage from all the satellites
// known to the storagenode and stores information into db
func (cache *Cache) CacheSpaceUsage(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// get current month edges
	startDate, endDate := date.MonthBoundary(time.Now().UTC())

	var cacheSpaceErr errs.Group
	for _, satellite := range cache.trust.GetSatellites(ctx) {
		spaceUsages, err := cache.service.GetDailyStorageUsage(ctx, satellite, startDate, endDate)
		if err != nil {
			cacheSpaceErr.Add(NodeStatsCacheErr.Wrap(err))
			continue
		}

		err = cache.db.StorageUsage.Store(ctx, spaceUsages)
		if err != nil {
			cacheSpaceErr.Add(NodeStatsCacheErr.Wrap(err))
			continue
		}
	}

	return cacheSpaceErr.Err()
}

// Close closes underlying cycles
func (cache *Cache) Close() error {
	defer mon.Task()(nil)(nil)
	cache.reputationCycle.Close()
	cache.storageCycle.Close()
	return nil
}
