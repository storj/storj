// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"sync"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type addressInfo struct {
	address   string
	lastIP    string
	transport pb.NodeTransport
}

// CombinedCache is a simple caching mechanism for overlaycache updates. It
// provdes methods to help reduce calls to UpdateAddress and UpdateTime, but can
// be extended for other calls in the future.
type CombinedCache struct {
	DB
	addressLock  sync.RWMutex
	addressCache map[storj.NodeID]*addressInfo

	keyLock *sync2.KeyLock
}

// NewCombinedCache instantiates a new CombinedCache
func NewCombinedCache(db DB) *CombinedCache {
	return &CombinedCache{
		DB:           db,
		addressCache: make(map[storj.NodeID]*addressInfo),
		keyLock:      sync2.NewKeyLock(),
	}
}

// UpdateAddress overrides the underlying db.UpdateAddress and provides a simple
// caching layer to reduce calls to the underlying db.
func (c *CombinedCache) UpdateAddress(ctx context.Context, info *pb.Node, defaults NodeSelectionConfig) (err error) {
	// Update internal cache and check if this call requires a db call

	if info == nil {
		return ErrEmptyNode
	}

	address := info.Address
	if address == nil {
		address = &pb.NodeAddress{}
	}

	c.addressLock.RLock()
	cached, ok := c.addressCache[info.Id]
	c.addressLock.RUnlock()

	if ok &&
		address.Address == cached.address &&
		address.Transport == cached.transport &&
		info.LastIp == cached.lastIP {

		return nil
	}

	// Acquire lock for this node ID. This prevents a concurrent db update to
	// this same node ID and guarantees the cache and database stay in sync
	unlockFunc := c.keyLock.Lock(info.Id)
	defer unlockFunc()

	err = c.DB.UpdateAddress(ctx, info, defaults)
	if err != nil {
		return err
	}

	c.addressLock.Lock()
	c.addressCache[info.Id] = &addressInfo{
		address:   address.Address,
		lastIP:    info.LastIp,
		transport: address.Transport,
	}
	c.addressLock.Unlock()

	return nil
}
