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
)

var (
	// mon = monkit.Package() //TODO: check whether this needs monitoring

	// Error is a general error class of this package
	Error = errs.Class("discovery error")
)

// Config loads on the configuration values for the cache
type Config struct {
	RefreshInterval time.Duration `help:"the interval at which the cache refreshes itself in seconds" default:"1s"`
	RefreshLimit    int           `help:"the amount of nodes refreshed at each interval" default:"100"`
}

// Refresh tracks the offset of the current refresh cycle
type Refresh struct {
	offset int64
}

// Discovery struct loads on cache, kad, and statdb
type Discovery struct {
	log     *zap.Logger
	cache   *overlay.Cache
	kad     *kademlia.Kademlia
	statdb  statdb.DB
	config  Config
	refresh Refresh
}

// New returns a new discovery service.
func New(logger *zap.Logger, ol *overlay.Cache, kad *kademlia.Kademlia, stat statdb.DB, config Config) *Discovery {
	return &Discovery{
		log:    logger,
		cache:  ol,
		kad:    kad,
		statdb: stat,
		config: config,
		refresh: Refresh{
			offset: 0,
		},
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
	ticker := time.NewTicker(discovery.config.RefreshInterval)
	defer ticker.Stop()

	for {
		err := discovery.Refresh(ctx)
		if err != nil {
			discovery.log.Error("Error with cache refresh: ", zap.Error(err))
		}

		err = discovery.Discovery(ctx)
		if err != nil {
			discovery.log.Error("Error with cache discovery: ", zap.Error(err))
		}

		select {
		case <-ticker.C: // redo
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Refresh updates the cache db with the current DHT.
// We currently do not penalize nodes that are unresponsive,
// but should in the future.
func (discovery *Discovery) Refresh(ctx context.Context) error {
	nodes := discovery.kad.Seen()
	for _, v := range nodes {
		if err := discovery.cache.Put(ctx, v.Id, *v); err != nil {
			return err
		}
	}

	list, more, err := discovery.cache.Paginate(ctx, discovery.refresh.offset, discovery.config.RefreshLimit)
	if err != nil {
		return Error.Wrap(err)
	}

	// more means there are more rows to page through in the cache
	if more == false {
		discovery.refresh.offset = 0
	} else {
		discovery.refresh.offset = discovery.refresh.offset + int64(len(list))
	}

	for _, node := range list {
		ping, err := discovery.kad.Ping(ctx, *node)
		if err != nil {
			discovery.log.Info("could not pinging node")
			_, err := discovery.statdb.UpdateUptime(ctx, ping.Id, false)
			if err != nil {
				discovery.log.Error("error updating node uptime in statdb")
			}
			err = discovery.cache.Delete(ctx, node.Id)
			if err != nil {
				discovery.log.Error("error deleting unresponsive node from cache:", zap.Error(err))
			}
			continue
		}

		_, err = discovery.statdb.UpdateUptime(ctx, ping.Id, true)
		if err != nil {
			discovery.log.Error("error updating node uptime in statdb")
		}
		err = discovery.cache.Put(ctx, ping.Id, ping)
		if err != nil {
			discovery.log.Error("error putting node into cache")
		}
	}

	return nil
}

// Bootstrap walks the initialized network and populates the cache
func (discovery *Discovery) Bootstrap(ctx context.Context) error {
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
func (discovery *Discovery) Discovery(ctx context.Context) error {
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
func (discovery *Discovery) Walk(ctx context.Context) error {
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
