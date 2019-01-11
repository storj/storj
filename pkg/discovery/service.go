// Copyright (C) 2018 Storj Labs, Inc.
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
	// DiscoveryError is a general error class of this package
	DiscoveryError = errs.Class("discovery error")
)

// Discovery struct loads on cache, kad, and statdb
type Discovery struct {
	log    *zap.Logger
	cache  *overlay.Cache
	kad    *kademlia.Kademlia
	statdb statdb.DB

	refreshInterval time.Duration
}

// New returns a new discovery service.
func New(logger *zap.Logger, ol *overlay.Cache, kad *kademlia.Kademlia, stat statdb.DB, refreshInterval time.Duration) *Discovery {
	return &Discovery{
		log:             logger,
		cache:           ol,
		kad:             kad,
		statdb:          stat,
		refreshInterval: refreshInterval,
	}
}

// NewDiscovery Returns a new Discovery instance with cache, kad, and statdb loaded on
func NewDiscovery(logger *zap.Logger, ol *overlay.Cache, kad *kademlia.Kademlia, stat statdb.DB) *Discovery {
	return &Discovery{
		log:    logger,
		cache:  ol,
		kad:    kad,
		statdb: stat,
	}
}

// Close closes resources
func (discovery *Discovery) Close() error { return nil }

// Run runs the discovery service
func (discovery *Discovery) Run(ctx context.Context) error {
	ticker := time.NewTicker(discovery.refreshInterval)
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
func (d *Discovery) Refresh(ctx context.Context) error {
	// TODO(coyle): make refresh work by looking on the network for new ndoes
	nodes := d.kad.Seen()

	for _, v := range nodes {
		if err := d.cache.Put(ctx, v.Id, *v); err != nil {
			return err
		}
	}

	return nil
}

// Bootstrap walks the initialized network and populates the cache
func (d *Discovery) Bootstrap(ctx context.Context) error {
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
func (d *Discovery) Discovery(ctx context.Context) error {
	r, err := randomID()
	if err != nil {
		return DiscoveryError.Wrap(err)
	}
	_, err = d.kad.FindNode(ctx, r)
	if err != nil && !kademlia.NodeNotFound.Has(err) {
		return DiscoveryError.Wrap(err)
	}
	return nil
}

// Walk iterates over each node in each bucket to traverse the network
func (d *Discovery) Walk(ctx context.Context) error {
	// TODO: This should walk the cache, rather than be a duplicate of refresh
	return nil
}

func randomID() (storj.NodeID, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return storj.NodeID{}, DiscoveryError.Wrap(err)
	}
	return storj.NodeIDFromBytes(b)
}
