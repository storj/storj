// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"sync"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

// CachedPublicProjectIDs wraps the database layer to provide caching.
// It now includes a map for the cache and a sync.RWMutex for thread safety.
type CachedPublicProjectIDs struct {
	db    console.Projects
	mu    sync.RWMutex
	cache map[uuid.UUID]uuid.UUID
}

// NewCachedPublicProjectIDs initializes the struct and the internal cache map.
func NewCachedPublicProjectIDs(db console.Projects) *CachedPublicProjectIDs {
	return &CachedPublicProjectIDs{
		db:    db,
		cache: make(map[uuid.UUID]uuid.UUID),
	}
}

// GetPublicID retrieves a public project ID, checking the cache first
// before falling back to a database lookup. The result is then cached.
func (p *CachedPublicProjectIDs) GetPublicID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	// Attempt a fast read with a read-lock.
	p.mu.RLock()
	publicID, ok := p.cache[id]
	p.mu.RUnlock()
	if ok {
		return publicID, nil
	}

	// If not found, acquire a full write-lock to populate the cache.
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check if another goroutine populated the cache while we waited for the lock.
	if publicID, ok = p.cache[id]; ok {
		return publicID, nil
	}

	// If still not in the cache, call the database.
	publicID, err := p.db.GetPublicID(ctx, id)
	if err != nil {
		return uuid.UUID{}, err
	}

	// Store the new value in the cache.
	p.cache[id] = publicID

	return publicID, nil
}

var _ PublicProjectIDer = &CachedPublicProjectIDs{}
