// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package discovery

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/overlay"
)

var (
	mon = monkit.Package()

	// Error is a general error class of this package
	Error = errs.Class("discovery error")
)

// Config loads on the configuration values for the cache
type Config struct {
	RefreshInterval    time.Duration `help:"the interval at which the cache refreshes itself in seconds" default:"1s"`
	RefreshLimit       int           `help:"the amount of nodes read from the overlay in a single pagination call" default:"100"`
	RefreshConcurrency int           `help:"the amount of nodes refreshed in parallel" default:"8"`
}

// Discovery struct loads on cache, kad
//
// architecture: Chore
type Discovery struct {
	log     *zap.Logger
	cache   *overlay.Service
	contact *contact.Service

	refreshLimit       int
	refreshConcurrency int

	Refresh sync2.Cycle
}

// New returns a new discovery service.
func New(logger *zap.Logger, ol *overlay.Service, contact *contact.Service, config Config) *Discovery {
	discovery := &Discovery{
		log:                logger,
		cache:              ol,
		contact:            contact,
		refreshLimit:       config.RefreshLimit,
		refreshConcurrency: config.RefreshConcurrency,
	}

	discovery.Refresh.SetInterval(config.RefreshInterval)
	return discovery
}

// Close closes resources
func (discovery *Discovery) Close() error {
	discovery.Refresh.Close()
	return nil
}

// Run runs the discovery service
func (discovery *Discovery) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group
	discovery.Refresh.Start(ctx, &group, func(ctx context.Context) error {
		err := discovery.refresh(ctx)
		if err != nil {
			discovery.log.Error("error with cache refresh: ", zap.Error(err))
		}
		return nil
	})
	return group.Wait()
}

// refresh updates the cache db with the current DHT.
func (discovery *Discovery) refresh(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	limiter := sync2.NewLimiter(discovery.refreshConcurrency)

	var offset int64

	for {
		list, more, err := discovery.cache.PaginateQualified(ctx, offset, discovery.refreshLimit)
		if err != nil {
			return Error.Wrap(err)
		}

		if len(list) == 0 {
			break
		}

		offset += int64(len(list))

		for _, node := range list {
			node := node

			limiter.Go(ctx, func() {
				info, err := discovery.contact.FetchInfo(ctx, *node)
				if ctx.Err() != nil {
					return
				}

				if err != nil {
					discovery.log.Info("could not ping node", zap.Stringer("ID", node.Id), zap.Error(err))
					return
				}

				if _, err = discovery.cache.UpdateNodeInfo(ctx, node.Id, info); err != nil {
					discovery.log.Warn("could not update node info", zap.Stringer("ID", node.GetAddress()))
				}
			})
		}

		if !more {
			break
		}
	}

	limiter.Wait()
	return nil
}
