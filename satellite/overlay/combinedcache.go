// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"sync"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// CombinedCache is a simple caching mechanism for overlaycache updates. It
// provdes methods to help reduce calls to UpdateAddress and UpdateTime, but can
// be extended for other calls in the future.
type CombinedCache struct {
	DB

	mu   sync.RWMutex
	info map[storj.NodeID]*cachedNodeInfo
}

type cachedNodeInfo struct {
	address   string
	transport pb.NodeTransport

	lastIP string
}

// NewCombinedCache instantiates a new CombinedCache
func NewCombinedCache(db DB) *CombinedCache {
	return &CombinedCache{
		DB:   db,
		info: make(map[storj.NodeID]*cachedNodeInfo),
	}
}

// UpdateAddress overrides the underlying db.UpdateAddress and provides a simple
// caching layer to reduce calls to the underlying db. The cache is guaranteed
// to match the values held in the database; however this code does not
// guarantee that concurrent UpdateAddress calls will be handled in any
// particular order.
func (cache *CombinedCache) UpdateAddress(ctx context.Context, info *pb.Node, defaults NodeSelectionConfig) (err error) {
	// Update internal cache and check if this call requires a db call
	if info == nil {
		return ErrEmptyNode
	}

	newInfo := newCachedInfo(info)
	if !cache.update(info.Id, newInfo) {
		return nil
	}

	err = cache.DB.UpdateAddress(ctx, info, defaults)
	if err != nil {
		return err
	}

	return nil
}

// update updates the cache information
func (cache *CombinedCache) update(id storj.NodeID, info *cachedNodeInfo) bool {
	cache.mu.RLock()
	cached, exists := cache.info[id]
	cache.mu.RUnlock()

	changed := !exists || !cached.Equal(info)
	if !changed {
		return false
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	cached, exists = cache.info[id]
	changed = !exists || !cached.Equal(info)
	if !changed {
		return false
	}

	cache.info[id] = info
	return true
}

// newCachedInfo creates cached info from the node
func newCachedInfo(info *pb.Node) *cachedNodeInfo {
	address := info.GetAddress()
	if address == nil {
		address = &pb.NodeAddress{}
	}

	return &cachedNodeInfo{
		address:   address.Address,
		transport: address.Transport,
		lastIP:    info.LastIp,
	}
}

// Equal compares it with existing value.
func (cached *cachedNodeInfo) Equal(newInfo *cachedNodeInfo) bool {
	return *cached == *newInfo
}
