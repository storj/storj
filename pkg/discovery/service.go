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
	"storj.io/storj/pkg/utils"
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

// StartRefresh kicks off a goroutine that refreshes the cache on an interval and returns an error
func (d *Discovery) StartRefresh(ctx context.Context) error {
	var errs []error
	go func() {
		t := time.NewTicker(500 * time.Millisecond).C
		for {
			<-t
			if err := d.Refresh(ctx); err != nil {
				d.log.Error(err.Error())
				errs = append(errs, err)
			}
		}
	}()
	return utils.CombineErrors(errs...)
}

// Refresh updates the cache db with the current DHT.
// We currently do not penalize nodes that are unresponsive,
// but should in the future.
func (d *Discovery) Refresh(ctx context.Context) error {
	nodes := d.kad.Seen()

	for _, node := range nodes {
		if _, err := d.kad.Ping(ctx, *node); err != nil {
			// fail ping refresh
			_, err = d.statdb.UpdateUptime(ctx, node.Id, false)
			if err != nil {
				d.log.Error("couldn't update uptime for node")
			}
		} else {
			// succeed ping refresh
			_, err = d.statdb.UpdateUptime(ctx, node.Id, true)
			if err != nil {
				d.log.Error("couldn't update uptime for node")
			}
		}
	}

	return nil
}

// Bootstrap populates the cache with the nodes from Kademlia#Seen()
func (d *Discovery) Bootstrap(ctx context.Context) error {
	nodes := d.kad.Seen()

	for _, v := range nodes {
		if err := d.cache.Put(ctx, v.Id, *v); err != nil {
			return err
		}
	}

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
