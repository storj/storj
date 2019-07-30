// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
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

// UpdateCache is a simple caching mechanism for overlaycache updates. It
// provdes methods to help reduce calls to UpdateAddress and UpdateTime, but can
// be extended for other calls in the future.
type UpdateCache struct {
	addressLock  sync.RWMutex
	addressCache map[storj.NodeID]*addressInfo

	uptimeLock          sync.RWMutex
	uptimeCache         map[storj.NodeID]*uptimeInfo
	uptimeFlushInterval time.Duration
}

// NewUpdateCache instantiates a new UpdateCache
func NewUpdateCache(uptimeFlushInterval time.Duration) *UpdateCache {
	return &UpdateCache{
		addressCache:        make(map[storj.NodeID]*addressInfo),
		uptimeCache:         make(map[storj.NodeID]*uptimeInfo),
		uptimeFlushInterval: uptimeFlushInterval,
	}
}

// SetAndCompareAddress returns true if the address should be updated in the
// underlying db, and false if not. It also keeps the internal cache up-to-date.
func (c *UpdateCache) SetAndCompareAddress(node *pb.Node) bool {
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
func (c *UpdateCache) GetNodeStats(nodeID storj.NodeID, isUp bool) (stats *NodeStats) {
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
func (c *UpdateCache) SetNodeStats(nodeID storj.NodeID, isUp bool, stats *NodeStats) {
	c.uptimeLock.Lock()
	c.uptimeCache[nodeID] = &uptimeInfo{
		isUp:       isUp,
		lastUptime: time.Now(),
		stats:      stats,
	}
	c.uptimeLock.Unlock()
}
