// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/private/date"
	"storj.io/storj/storagenode/payouts"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
)

// Config defines nodestats cache configuration.
type Config struct {
	MaxSleep       time.Duration `help:"maximum duration to wait before requesting data" releaseDefault:"300s" devDefault:"1s"`
	ReputationSync time.Duration `help:"how often to sync reputation" releaseDefault:"4h" devDefault:"1m" deprecated:"use --reputation.interval" hidden:"true"`
	StorageSync    time.Duration `help:"how often to sync storage" releaseDefault:"12h" devDefault:"2m"`
}

// CacheStorage encapsulates cache DBs.
type CacheStorage struct {
	StorageUsage storageusage.DB
	Payout       payouts.DB
	Pricing      pricing.DB
}

// Cache runs cache loop and stores reputation stats and storage usage into db.
//
// architecture: Chore
type Cache struct {
	log *zap.Logger

	db             CacheStorage
	service        *Service
	payoutEndpoint *payouts.Endpoint
	trust          *trust.Pool

	maxSleep time.Duration
	Storage  *sync2.Cycle

	UsageStat *UsageStat
}

// NewCache creates new caching service instance.
func NewCache(log *zap.Logger, config Config, db CacheStorage, service *Service,
	payoutEndpoint *payouts.Endpoint, trust *trust.Pool) *Cache {

	cache := &Cache{
		log:            log,
		db:             db,
		service:        service,
		payoutEndpoint: payoutEndpoint,
		trust:          trust,
		maxSleep:       config.MaxSleep,
		Storage:        sync2.NewCycle(config.StorageSync),

		UsageStat: NewUsageStat(),
	}
	mon.Chain(cache.UsageStat)
	return cache
}

// Run runs loop.
func (cache *Cache) Run(ctx context.Context) error {
	var group errgroup.Group

	err := cache.satelliteLoop(ctx, func(satelliteID storj.NodeID) error {
		stubHistory, err := cache.payoutEndpoint.GetAllPaystubs(ctx, satelliteID)
		if err != nil {
			return err
		}

		for i := 0; i < len(stubHistory); i++ {
			err := cache.db.Payout.StorePayStub(ctx, stubHistory[i])
			if err != nil {
				return err
			}
		}

		paymentHistory, err := cache.payoutEndpoint.GetAllPayments(ctx, satelliteID)
		if err != nil {
			return err
		}

		for j := 0; j < len(paymentHistory); j++ {
			err := cache.db.Payout.StorePayment(ctx, paymentHistory[j])
			if err != nil {
				return err
			}
		}

		pricingModel, err := cache.service.GetPricingModel(ctx, satelliteID)
		if err != nil {
			return err
		}
		return cache.db.Pricing.Store(ctx, *pricingModel)
	})
	if err != nil {
		cache.log.Error("Get pricing-model/join date failed", zap.Error(err))
	}

	cache.Storage.Start(ctx, &group, func(ctx context.Context) error {
		if err := cache.sleep(ctx); err != nil {
			return err
		}

		err := cache.CacheSpaceUsage(ctx)
		if err != nil {
			cache.log.Error("Get disk space usage query failed", zap.Error(err))
		}

		err = cache.CacheHeldAmount(ctx)
		if err != nil {
			cache.log.Error("Get held amount query failed", zap.Error(err))
		}

		return nil
	})

	return group.Wait()
}

// UsageStat caches last space usage value for each satellite, to make it available for monkit.
type UsageStat struct {
	mu        sync.Mutex
	usedBytes map[storj.NodeID]float64
}

// NewUsageStat initializes a UsageState.
func NewUsageStat() *UsageStat {
	return &UsageStat{
		usedBytes: make(map[storj.NodeID]float64),
	}
}

// Stats implements monkit.StatSource.
func (u *UsageStat) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for satellite, space := range u.usedBytes {
		cb(monkit.NewSeriesKey("satellite_usage").WithTag("satellite", satellite.String()), "used_bytes", space)
	}
}

// Update updates the cached value.
func (u *UsageStat) Update(satellite storj.NodeID, usedSpace float64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.usedBytes[satellite] = usedSpace
}

var _ monkit.StatSource = &UsageStat{}

// CacheSpaceUsage queries disk space usage from all the satellites
// known to the storagenode and stores information into db.
func (cache *Cache) CacheSpaceUsage(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// get current month edges
	startDate, endDate := date.MonthBoundary(time.Now().UTC())
	// start from last day of previous month
	startDate = startDate.AddDate(0, 0, -1)

	return cache.satelliteLoop(ctx, func(satellite storj.NodeID) error {
		spaceUsages, err := cache.service.GetDailyStorageUsage(ctx, satellite, startDate, endDate)
		if err != nil {
			return err
		}

		// update monkit cache
		if len(spaceUsages) > 1 {
			lastRec := spaceUsages[len(spaceUsages)-1]
			cache.UsageStat.Update(satellite, lastRec.AtRestTotal/lastRec.IntervalEndTime.Sub(spaceUsages[len(spaceUsages)-2].IntervalEndTime).Hours())
		}

		err = cache.db.StorageUsage.Store(ctx, spaceUsages)
		if err != nil {
			return err
		}

		return nil
	})
}

// CacheHeldAmount queries held amount stats and payments from
// all the satellites known to the storagenode and stores info into db.
func (cache *Cache) CacheHeldAmount(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return cache.satelliteLoop(ctx, func(satellite storj.NodeID) error {
		now := time.Now().String()
		yearAndMonth, err := date.PeriodToTime(now)
		if err != nil {
			return err
		}

		previousMonth := yearAndMonth.AddDate(0, -1, 0).String()
		payStub, err := cache.payoutEndpoint.GetPaystub(ctx, satellite, previousMonth)
		if err != nil {
			if payouts.ErrNoPayStubForPeriod.Has(err) {
				return nil
			}

			cache.log.Error("payouts err", zap.String("satellite", satellite.String()))
			return err
		}

		if payStub != nil {
			if err = cache.db.Payout.StorePayStub(ctx, *payStub); err != nil {
				return err
			}
		}

		payment, err := cache.payoutEndpoint.GetPayment(ctx, satellite, previousMonth)
		if err != nil {
			return err
		}

		if payment != nil {
			if err = cache.db.Payout.StorePayment(ctx, *payment); err != nil {
				return err
			}
		}

		return nil
	})
}

// sleep for random interval in [0;maxSleep)
// returns error if context was cancelled.
func (cache *Cache) sleep(ctx context.Context) error {
	if cache.maxSleep <= 0 {
		return nil
	}

	jitter := time.Duration(rand.Int63n(int64(cache.maxSleep)))
	if !sync2.Sleep(ctx, jitter) {
		return ctx.Err()
	}

	return nil
}

// satelliteLoop loops over all satellites from trust pool executing provided fn, caching errors if occurred,
// on each step checks if context has been cancelled.
func (cache *Cache) satelliteLoop(ctx context.Context, fn func(id storj.NodeID) error) error {
	var groupErr errs.Group
	for _, satellite := range cache.trust.GetSatellites(ctx) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		groupErr.Add(fn(satellite))
	}

	return groupErr.Err()
}

// Close closes underlying cycles.
func (cache *Cache) Close() error {
	defer mon.Task()(nil)(nil)
	cache.Storage.Close()
	return nil
}
