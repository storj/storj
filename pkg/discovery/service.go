// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package discovery

import (
	"context"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/statdb"
)

// Discovery struct loads on cache, kad, and statdb
type Discovery struct {
	cache  *overlay.Cache
	kad    *kademlia.Kademlia
	statdb statdb.DB
}

// NewDiscovery Returns a new Discovery instance with cache, kad, and statdb loaded on
func NewDiscovery(ol *overlay.Cache, kad *kademlia.Kademlia, stat statdb.DB) *Discovery {
	return &Discovery{
		cache:  ol,
		kad:    kad,
		statdb: stat,
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
