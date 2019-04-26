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

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var (
	// mon = monkit.Package() //TODO: check whether this needs monitoring

	// Error is a general error class of this package
	Error = errs.Class("discovery error")
)

// Config loads on the configuration values for the cache
type Config struct {
	RefreshInterval   time.Duration `help:"the interval at which the cache refreshes itself in seconds" default:"1s"`
	GraveyardInterval time.Duration `help:"the interval at which the the graveyard tries to resurrect nodes" default:"30s"`
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
	Graveyard sync2.Cycle
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
	discovery.Graveyard.SetInterval(config.GraveyardInterval)
	discovery.Discovery.SetInterval(config.DiscoveryInterval)

	return discovery
}

// Close closes resources
func (discovery *Discovery) Close() error {
	discovery.Refresh.Close()
	discovery.Graveyard.Close()
	discovery.Discovery.Close()
	return nil
}

// Run runs the discovery service
func (discovery *Discovery) Run(ctx context.Context) error {
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
// We currently do not penalize nodes that are unresponsive,
// but should in the future.
func (discovery *Discovery) refresh(ctx context.Context) error {
	nodes := discovery.kad.Seen()
	for _, node := range nodes {
		ping, err := discovery.kad.Ping(ctx, *node)
		if err != nil {
			discovery.log.Info("could not ping node", zap.String("ID", node.Id.String()), zap.Error(err))
			_, err := discovery.cache.UpdateUptime(ctx, node.Id, false)
			if err != nil {
				discovery.log.Error("could not update node uptime in cache", zap.String("ID", node.Id.String()), zap.Error(err))
			}
			continue
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		// update wallet with correct info
		info, err := discovery.kad.FetchInfo(ctx, *node)
		if err != nil {
			discovery.log.Warn("could not fetch node info", zap.String("ID", ping.GetAddress().String()))
			continue
		}

		if (info.Type == pb.NodeType_INVALID) || (info.Type == pb.NodeType_BOOTSTRAP) || (info.Type == pb.NodeType_SATELLITE) {
			discovery.log.Warn("node info not needed to be added", zap.String("ID", ping.GetAddress().String()))
			continue
		}

		err = discovery.cache.Put(ctx, ping.Id, ping)
		if err != nil {
			zap.L().Debug("error updating uptime for node", zap.Error(err))
		}

		_, err = discovery.cache.UpdateUptime(ctx, ping.Id, true)
		if err != nil {
			zap.L().Debug("error updating node connection info", zap.Error(err))
		}

		_, err = discovery.cache.UpdateNodeInfo(ctx, ping.Id, info)
		if err != nil {
			discovery.log.Warn("could not update node info", zap.String("ID", ping.GetAddress().String()))
		}
	}

	list, more, err := discovery.cache.Paginate(ctx, discovery.refreshOffset, discovery.refreshLimit)
	if err != nil {
		return Error.Wrap(err)
	}

	// more means there are more rows to page through in the cache
	if more == false {
		discovery.refreshOffset = 0
	} else {
		discovery.refreshOffset = discovery.refreshOffset + int64(len(list))
	}

	for _, node := range list {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		ping, err := discovery.kad.Ping(ctx, node.Node)
		if err != nil {
			discovery.log.Info("could not ping node", zap.String("ID", node.Id.String()), zap.Error(err))
			_, err := discovery.cache.UpdateUptime(ctx, node.Id, false)
			if err != nil {
				discovery.log.Error("could not update node uptime in cache", zap.String("ID", node.Id.String()), zap.Error(err))
			}
			continue
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		_, err = discovery.cache.UpdateUptime(ctx, ping.Id, true)
		if err != nil {
			discovery.log.Error("could not update node uptime in cache", zap.String("ID", ping.Id.String()), zap.Error(err))
		}
	}

	return nil
}

// Discovery runs lookups for random node ID's to find new nodes in the network
func (discovery *Discovery) discover(ctx context.Context) error {
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

// Walk iterates over each node in each bucket to traverse the network
func (discovery *Discovery) walk(ctx context.Context) error {
	// TODO: This should walk the cache, rather than be a duplicate of refresh
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
