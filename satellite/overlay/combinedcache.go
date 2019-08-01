// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"sync"
	"time"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type addressInfo struct {
	address   string
	lastIP    string
	transport pb.NodeTransport
}

type uptimeInfo struct {
	isUp       bool
	lastUptime time.Time
	stats      *NodeStats
}

// CombinedCache is a simple caching mechanism for overlaycache updates. It
// provdes methods to help reduce calls to UpdateAddress and UpdateTime, but can
// be extended for other calls in the future.
type CombinedCache struct {
	DB
	addressLock  sync.RWMutex
	addressCache map[storj.NodeID]*addressInfo

	uptimeLock          sync.RWMutex
	uptimeCache         map[storj.NodeID]*uptimeInfo
	uptimeFlushInterval time.Duration
}

// NewCombinedCache instantiates a new CombinedCache
func NewCombinedCache(db DB, uptimeFlushInterval time.Duration) *CombinedCache {
	return &CombinedCache{
		DB:                  db,
		addressCache:        make(map[storj.NodeID]*addressInfo),
		uptimeCache:         make(map[storj.NodeID]*uptimeInfo),
		uptimeFlushInterval: uptimeFlushInterval,
	}
}

// UpdateUptime overrides the underlying db.UpdateUptime and provides a simple
// caching layer to reduce calls to the underlying db.
func (c *CombinedCache) UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool, lambda, weight, uptimeDQ float64) (stats *NodeStats, err error) {
	// First check the internal cache. If it returns stats, use them.
	stats = c.GetNodeStats(nodeID, isUp)
	if stats != nil {
		return stats, nil
	}
	stats, err = c.DB.UpdateUptime(ctx, nodeID, isUp, lambda, weight, uptimeDQ)
	if err != nil {
		return nil, err
	}
	// Refresh internal stats
	c.SetNodeStats(nodeID, isUp, stats)
	return stats, nil
}

// UpdateAddress overrides the underlying db.UpdateAddress and provides a simple
// caching layer to reduce calls to the underlying db.
func (c *CombinedCache) UpdateAddress(ctx context.Context, info *pb.Node, defaults NodeSelectionConfig) (err error) {
	// Update internal cache and check if this call requires a db call
	if !c.SetAndCompareAddress(info) {
		return nil
	}
	return c.DB.UpdateAddress(ctx, info, defaults)
}

// SetAndCompareAddress returns true if the address should be updated in the
// underlying db, and false if not. It also keeps the internal cache up-to-date.
func (c *CombinedCache) SetAndCompareAddress(node *pb.Node) bool {
	// There's nothing we can do with a nil node, or nil address, we're just
	// gonna say don't update it
	if node == nil {
		return false
	}

	address := node.Address
	if address == nil {
		address = &pb.NodeAddress{}
	}

	c.addressLock.RLock()
	cached, ok := c.addressCache[node.Id]
	c.addressLock.RUnlock()

	// If it's not in our cache, add it and say to update
	if !ok ||
		address.Address != cached.address ||
		address.Transport != cached.transport ||
		node.LastIp != cached.lastIP {

		c.addressLock.Lock()
		c.addressCache[node.Id] = &addressInfo{
			address:   address.Address,
			lastIP:    node.LastIp,
			transport: address.Transport,
		}
		c.addressLock.Unlock()
		return true
	}

	return false
}

// GetNodeStats returns cached NodeStats for the supplied nodeID. If the
// returned stats are not nil the caller should use them as up-to-date stats. If
// nil, the called should re-cache NodeStats for this nodeID.
func (c *CombinedCache) GetNodeStats(nodeID storj.NodeID, isUp bool) (stats *NodeStats) {
	c.uptimeLock.RLock()
	cached, ok := c.uptimeCache[nodeID]
	c.uptimeLock.RUnlock()

	if !ok ||
		cached.isUp != isUp ||
		time.Since(cached.lastUptime) > c.uptimeFlushInterval {

		return nil
	}

	return cached.stats
}

// SetNodeStats will cache the supplied stats, keyed by nodeID
func (c *CombinedCache) SetNodeStats(nodeID storj.NodeID, isUp bool, stats *NodeStats) {
	c.uptimeLock.Lock()
	c.uptimeCache[nodeID] = &uptimeInfo{
		isUp:       isUp,
		lastUptime: time.Now(),
		stats:      stats,
	}
	c.uptimeLock.Unlock()
}
