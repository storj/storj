// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// ReliabilityCache caches the reliable nodes for the specified staleness duration
// and updates automatically from overlay.
//
// ReliabilityCache is NOT safe for concurrent use.
type ReliabilityCache struct {
	overlay    *overlay.Cache
	staleness  time.Duration
	lastUpdate time.Time
	reliable   map[storj.NodeID]struct{}
}

// NewReliabilityCache creates a new reliability checking cache.
func NewReliabilityCache(overlay *overlay.Cache, staleness time.Duration) *ReliabilityCache {
	return &ReliabilityCache{
		overlay:   overlay,
		staleness: staleness,
		reliable:  map[storj.NodeID]struct{}{},
	}
}

// LastUpdate returns when the cache was last updated.
func (cache *ReliabilityCache) LastUpdate() time.Time { return cache.lastUpdate }

// MissingPieces returns piece indices that are unreliable with the given staleness period.
func (cache *ReliabilityCache) MissingPieces(ctx context.Context, created time.Time, pieces []*pb.RemotePiece) ([]int32, error) {
	if created.After(cache.lastUpdate) || time.Since(cache.lastUpdate) > cache.staleness {
		err := cache.Refresh(ctx)
		if err != nil {
			return nil, err
		}
	}

	var unreliable []int32
	for _, piece := range pieces {
		if _, ok := cache.reliable[piece.NodeId]; !ok {
			unreliable = append(unreliable, piece.PieceNum)
		}
	}
	return unreliable, nil
}

// Refresh refreshes the cache.
func (cache *ReliabilityCache) Refresh(ctx context.Context) error {
	for id := range cache.reliable {
		delete(cache.reliable, id)
	}

	cache.lastUpdate = time.Now()

	nodes, err := cache.overlay.Reliable(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	for _, id := range nodes {
		cache.reliable[id] = struct{}{}
	}

	return nil
}
