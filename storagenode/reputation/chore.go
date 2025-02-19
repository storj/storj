// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"
	"math/rand"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/sync2"
)

var (
	// ErrReputationService is a base error class for reputation service.
	ErrReputationService = errs.Class("reputation")

	mon = monkit.Package()
)

// Config defines reputation service configuration.
type Config struct {
	MaxSleep time.Duration `help:"maximum duration to wait before requesting data" releaseDefault:"300s" devDefault:"1s"`
	Interval time.Duration `help:"how often to sync reputation" releaseDefault:"4h" devDefault:"1m"`
	Cache    bool          `help:"store reputation stats in cache" releaseDefault:"true" devDefault:"true"`
}

// Chore periodically fetches reputation stats from satellites.
type Chore struct {
	log *zap.Logger

	service *Service
	config  Config

	Loop *sync2.Cycle
}

// NewChore creates new reputation chore instance.
func NewChore(log *zap.Logger, service *Service, config Config) *Chore {
	return &Chore{
		log:     log,
		service: service,
		config:  config,
		Loop:    sync2.NewCycle(config.Interval),
	}
}

// Run runs the reputation service.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		if err := chore.sleep(ctx); err != nil {
			return err
		}

		if err := chore.RunOnce(ctx); err != nil {
			chore.log.Error("reputation chore failed", zap.Error(err))
		}

		return nil
	})
}

// RunOnce runs the reputation service once.
func (chore *Chore) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var groupErr errs.Group
	for _, satellite := range chore.service.trust.GetSatellites(ctx) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := chore.GetAndCacheStats(ctx, satellite); err != nil {
			groupErr.Add(err)
		}
	}

	return groupErr.Err()
}

// GetAndCacheStats retrieves reputation stats from particular satellite and stores it in cache.
func (chore *Chore) GetAndCacheStats(ctx context.Context, satellite storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	stats, err := chore.service.GetStats(ctx, satellite)
	if err != nil {
		return err
	}

	satelliteTag := monkit.NewSeriesTag("satellite", satellite.String())
	mon.Counter("reputation_audits_total", satelliteTag).Set(stats.Audit.TotalCount)
	mon.Counter("reputation_audits_success", satelliteTag).Set(stats.Audit.SuccessCount)
	mon.FloatVal("reputation_online_score", satelliteTag).Observe(stats.OnlineScore)
	mon.FloatVal("reputation_score", satelliteTag).Observe(stats.Audit.Score)
	mon.FloatVal("reputation_unknown_score", satelliteTag).Observe(stats.Audit.UnknownScore)
	suspensionAge := mon.DurationVal("reputation_suspension_age", satelliteTag)
	if stats.SuspendedAt == nil {
		suspensionAge.Observe(0)
	} else {
		suspensionAge.Observe(time.Since(*stats.SuspendedAt))
	}
	dqAge := mon.DurationVal("reputation_dq_age", satelliteTag)
	if stats.DisqualifiedAt == nil {
		dqAge.Observe(0)
	} else {
		dqAge.Observe(time.Since(*stats.DisqualifiedAt))
	}

	if chore.config.Cache {
		if err = chore.service.Store(ctx, *stats, satellite); err != nil {
			chore.log.Error("failed to store reputation", zap.Error(err))
			return err
		}
	}

	return nil
}

// sleep for random interval in [0;maxSleep)
// returns error if context was cancelled.
func (chore *Chore) sleep(ctx context.Context) error {
	if chore.config.MaxSleep <= 0 {
		return nil
	}

	jitter := time.Duration(rand.Int63n(int64(chore.config.MaxSleep)))
	if !sync2.Sleep(ctx, jitter) {
		return ctx.Err()
	}

	return nil
}

// Close closes the reputation service.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
