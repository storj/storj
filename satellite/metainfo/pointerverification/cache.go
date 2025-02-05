// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerverification

import (
	"context"
	"sync"

	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
)

// IdentityCache implements caching of *identity.PeerIdentity.
type IdentityCache struct {
	db overlay.PeerIdentities

	mu     sync.RWMutex
	cached map[storj.NodeID]signing.Signee
}

// NewIdentityCache returns an IdentityCache.
func NewIdentityCache(db overlay.PeerIdentities) *IdentityCache {
	return &IdentityCache{
		db:     db,
		cached: map[storj.NodeID]signing.Signee{},
	}
}

// GetCached returns the peer identity in the cache.
func (cache *IdentityCache) GetCached(ctx context.Context, id storj.NodeID) signing.Signee {
	defer mon.Task()(&ctx)(nil)

	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return cache.cached[id]
}

// GetUpdated returns the identity from database and updates the cache.
func (cache *IdentityCache) GetUpdated(ctx context.Context, id storj.NodeID) (_ signing.Signee, err error) {
	defer mon.Task()(&ctx)(&err)

	identity, err := cache.db.Get(ctx, id)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	signee := signing.SigneeFromPeerIdentity(identity)

	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.cached[id] = signee

	return signee, nil
}

// EnsureCached loads any missing identity into cache.
func (cache *IdentityCache) EnsureCached(ctx context.Context, nodes []storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	missing := []storj.NodeID{}

	cache.mu.RLock()
	for _, node := range nodes {
		if _, ok := cache.cached[node]; !ok {
			missing = append(missing, node)
		}
	}
	cache.mu.RUnlock()

	if len(missing) == 0 {
		return nil
	}

	// There might be a race during updating, however we'll "reupdate" later if there's a failure.
	// The common path doesn't end up here.

	identities, err := cache.db.BatchGet(ctx, missing)
	if err != nil {
		return Error.Wrap(err)
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	for _, identity := range identities {
		cache.cached[identity.ID] = signing.SigneeFromPeerIdentity(identity)
	}

	return nil
}
