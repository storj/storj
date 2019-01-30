// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package discovery

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
)

var (
	// mon = monkit.Package() //TODO: check whether this needs monitoring

	// Error is a general error class of this package
	Error = errs.Class("discovery error")
)

// Config loads on the configuration values for the cache
type Config struct {
	RefreshInterval   time.Duration `help:"the interval at which the cache refreshes itself in seconds" default:"1s"`
	GraveyardInterval time.Duration `help:"the interval at which the satellite attempts to contact previously unresponsive nodes" default:"1h"`
	DiscoveryInterval time.Duration `help:"the interval at which the satellite attempts to find new nodes via random node ID lookups" default:"1s"`
	RefreshLimit      int           `help:"the amount of nodes refreshed at each interval" default:"100"`
}

// Discovery struct loads on cache, kad, and statdb
type Discovery struct {
	log    *zap.Logger
	cache  *overlay.Cache
	kad    *kademlia.Kademlia
	statdb statdb.DB
	config Config

	// refreshOffset tracks the offset of the current refresh cycle
	refreshOffset int64
}

// New returns a new discovery service.
func New(logger *zap.Logger, ol *overlay.Cache, kad *kademlia.Kademlia, stat statdb.DB, config Config) *Discovery {
	return &Discovery{
		log:    logger,
		cache:  ol,
		kad:    kad,
		statdb: stat,
		config: config,

		refreshOffset: 0,
	}
}

// NewDiscovery Returns a new Discovery instance with cache, kad, and statdb loaded on
func NewDiscovery(logger *zap.Logger, ol *overlay.Cache, kad *kademlia.Kademlia, stat statdb.DB, config Config) *Discovery {
	return &Discovery{
		log:    logger,
		cache:  ol,
		kad:    kad,
		statdb: stat,
		config: config,
	}
}

// Close closes resources
func (discovery *Discovery) Close() error { return nil }

// Run runs the discovery service
func (discovery *Discovery) Run(ctx context.Context) error {
	refresh := time.NewTicker(discovery.config.RefreshInterval)
	graveyard := time.NewTicker(discovery.config.GraveyardInterval)
	discover := time.NewTicker(discovery.config.DiscoveryInterval)
	defer refresh.Stop()
	defer graveyard.Stop()
	defer discover.Stop()

	for {
		select {
		case <-refresh.C:
			discovery.log.Info("REFRESH STARTING")
			err := discovery.refresh(ctx)
			if err != nil {
				discovery.log.Error("Error with cache refresh: ", zap.Error(err))
			}
		case <-discover.C:
			discovery.log.Info("DISCOVERY STARTING")
			err := discovery.discover(ctx)
			if err != nil {
				discovery.log.Error("Error with cache discovery: ", zap.Error(err))
			}
		case <-graveyard.C:
			discovery.log.Info("GRAVEYARD RESURRECTION STARTING")
			err := discovery.graveyard(ctx)
			if err != nil {
				discovery.log.Error("graveyard resurrection failed")
			}
		case <-ctx.Done():
			discovery.log.Info("DISCOVERY DONE FIRED")
			return ctx.Err()
		default: // don't block
		}
	}
}

// refresh updates the cache db with the current DHT.
// We currently do not penalize nodes that are unresponsive,
// but should in the future.
func (discovery *Discovery) refresh(ctx context.Context) error {
	nodes := discovery.kad.Seen()
	for _, v := range nodes {
		if err := discovery.cache.Put(ctx, v.Id, *v); err != nil {
			return err
		}
	}

	list, more, err := discovery.cache.Paginate(ctx, discovery.refreshOffset, discovery.config.RefreshLimit)
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
		ping, err := discovery.kad.Ping(ctx, *node)
		if err != nil {
			discovery.log.Info("could not ping node")
			_, err := discovery.statdb.UpdateUptime(ctx, ping.Id, false)
			if err != nil {
				discovery.log.Error("could not update node uptime in statdb")
			}
			err = discovery.cache.Delete(ctx, node.Id)
			if err != nil {
				discovery.log.Error("deleting unresponsive node from cache:", zap.Error(err))
			}
			continue
		}

		_, err = discovery.statdb.UpdateUptime(ctx, ping.Id, true)
		if err != nil {
			discovery.log.Error("could not update node uptime in statdb")
		}
		err = discovery.cache.Put(ctx, ping.Id, ping)
		if err != nil {
			discovery.log.Error("could not put node into cache")
		}
	}

	return nil
}

// graveyard attempts to ping all nodes in the Seen() map from Kademlia and adds them to the cache
// if they respond. This is an attempt to resurrect nodes that may have gone offline in the last hour
// and were removed from the cache due to an unsuccessful response.
func (discovery *Discovery) graveyard(ctx context.Context) error {
	discovery.log.Debug("starting node graveyard resurrection")
	seen := discovery.kad.Seen()
	var errors utils.ErrorGroup

	for _, n := range seen {
		ping, err := discovery.kad.Ping(ctx, *n)
		if err != nil {
			discovery.log.Debug("could not ping node in graveyard check")
			// we don't want to report the ping error to ErrorGroup because it's to be expected here.
			continue
		}

		err = discovery.cache.Put(ctx, ping.Id, ping)
		if err != nil {
			discovery.log.Warn("could not update node uptime")
			errors.Add(err)
		}

		_, err = discovery.statdb.UpdateUptime(ctx, ping.Id, true)
		if err != nil {
			discovery.log.Warn("could not update node uptime")
			errors.Add(err)
		}
	}
	return errors.Finish()
}

// Bootstrap walks the initialized network and populates the cache
func (discovery *Discovery) bootstrap(ctx context.Context) error {
	// o := overlay.LoadFromContext(ctx)
	// kad := kademlia.LoadFromContext(ctx)
	// TODO(coyle): make Bootstrap work
	// look in our routing table
	// get every node we know about
	// ask every node for every node they know about
	// for each newly known node, ask those nodes for every node they know about
	// continue until no new nodes are found
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
