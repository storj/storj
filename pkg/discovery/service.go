// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package discovery

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/storj"
)

var (
	mon = monkit.Package()

	// Error is a general error class of this package
	Error = errs.Class("discovery error")
)

// Config loads on the configuration values for the cache
type Config struct {
	RefreshInterval   time.Duration `help:"the interval at which the cache refreshes itself in seconds" default:"1s"`
	DiscoveryInterval time.Duration `help:"the interval at which the satellite attempts to find new nodes via random node ID lookups" default:"1s"`
	RefreshLimit      int           `help:"the amount of nodes refreshed at each interval" default:"100"`
}

// Discovery struct loads on cache, kad
type Discovery struct {
	log   *zap.Logger
	cache *overlay.Cache
	kad   *kademlia.Kademlia

	// refreshOffset tracks the offset of the current refresh cycle
	refreshOffset int64
	refreshLimit  int

	Refresh   sync2.Cycle
	Discovery sync2.Cycle
}

// New returns a new discovery service.
func New(logger *zap.Logger, ol *overlay.Cache, kad *kademlia.Kademlia, config Config) *Discovery {
	discovery := &Discovery{
		log:   logger,
		cache: ol,
		kad:   kad,

		refreshOffset: 0,
		refreshLimit:  config.RefreshLimit,
	}

	discovery.Refresh.SetInterval(config.RefreshInterval)
	discovery.Discovery.SetInterval(config.DiscoveryInterval)

	return discovery
}

// Close closes resources
func (discovery *Discovery) Close() error {
	discovery.Refresh.Close()
	discovery.Discovery.Close()
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
	discovery.Discovery.Start(ctx, &group, func(ctx context.Context) error {
		err := discovery.discover(ctx)
		if err != nil {
			discovery.log.Error("error with cache discovery: ", zap.Error(err))
		}
		return nil
	})
	return group.Wait()
}

// refresh updates the cache db with the current DHT.
func (discovery *Discovery) refresh(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	list, more, err := discovery.cache.Paginate(ctx, discovery.refreshOffset, discovery.refreshLimit)
	if err != nil {
		return Error.Wrap(err)
	}

	// more means there are more rows to page through in the cache
	if more == false {
		discovery.refreshOffset = 0
	} else {
		discovery.refreshOffset += int64(len(list))
	}

	for _, node := range list {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if node.Disqualified != nil {
			discovery.log.Debug("skip disqualified node", zap.Stringer("ID", node.Id))
			continue
		}

		info, err := discovery.kad.FetchInfo(ctx, node.Node)

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			discovery.log.Info("could not ping node", zap.Stringer("ID", node.Id), zap.Error(err))
			_, err := discovery.cache.UpdateUptime(ctx, node.Id, false)
			if err != nil {
				discovery.log.Error("could not update node uptime in cache", zap.Stringer("ID", node.Id), zap.Error(err))
			}
			continue
		}

		// TODO: combine these into the same db call
		_, err = discovery.cache.UpdateUptime(ctx, node.Id, true)
		if err != nil {
			discovery.log.Error("could not update node uptime in cache", zap.Stringer("ID", node.Id), zap.Error(err))
		}
		_, err = discovery.cache.UpdateNodeInfo(ctx, node.Id, info)
		if err != nil {
			discovery.log.Warn("could not update node info", zap.Stringer("ID", node.GetAddress()))
		}
	}

	return nil
}

// Discovery runs lookups for random node ID's to find new nodes in the network
func (discovery *Discovery) discover(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	r, err := randomID()
	if err != nil {
		return Error.Wrap(err)
	}
	_, err = discovery.kad.FindNode(ctx, r)
	if err != nil && !kademlia.NodeNotFound.Has(err) {
		return Error.Wrap(err)
	}
	return nil
}

func randomID() (storj.NodeID, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return storj.NodeID{}, Error.Wrap(err)
	}
	return storj.NodeIDFromBytes(b)
}
