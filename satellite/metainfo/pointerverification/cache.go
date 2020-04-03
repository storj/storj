// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerverification

import (
	"context"
	"sync"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
)

// IdentityCache implements caching of *identity.PeerIdentity.
type IdentityCache struct {
	db overlay.PeerIdentities

	mu     sync.RWMutex
	cached map[storj.NodeID]*identity.PeerIdentity
}

// NewIdentityCache returns an IdentityCache.
func NewIdentityCache(db overlay.PeerIdentities) *IdentityCache {
	return &IdentityCache{
		db:     db,
		cached: map[storj.NodeID]*identity.PeerIdentity{},
	}
}

// GetCached returns the peer identity in the cache.
func (cache *IdentityCache) GetCached(ctx context.Context, id storj.NodeID) *identity.PeerIdentity {
	defer mon.Task()(&ctx)(nil)

	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return cache.cached[id]
}

// GetUpdated returns the identity from database and updates the cache.
func (cache *IdentityCache) GetUpdated(ctx context.Context, id storj.NodeID) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)

	identity, err := cache.db.Get(ctx, id)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.cached[id] = identity

	return identity, nil
}

// EnsureCached loads any missing identity into cache.
func (cache *IdentityCache) EnsureCached(ctx context.Context, pieces []*pb.RemotePiece) (err error) {
	defer mon.Task()(&ctx)(&err)

	missing := []storj.NodeID{}

	cache.mu.RLock()
	for _, piece := range pieces {
		if _, ok := cache.cached[piece.NodeId]; !ok {
			missing = append(missing, piece.NodeId)
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
		cache.cached[identity.ID] = identity
	}

	return nil
}
