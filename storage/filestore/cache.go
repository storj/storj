// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"context"
	"sync"
)

type spaceUsed struct {
	mu               sync.Mutex
	total            int64
	totalByNamespace map[string]int64
}

// InitCache initializes the cache with total values of current space usage
func (cache *spaceUsed) InitCache(ctx context.Context, newtotalSpaceUsed int64, newtotalSpaceUsedByNamespace map[string]int64) error {
	cache.mu.Lock()
	cache.total = newtotalSpaceUsed
	cache.totalByNamespace = newtotalSpaceUsedByNamespace
	cache.mu.Unlock()
	return nil
}

// SpaceUsedForPiecesLive returns the current total used space for
// all pieces content (not including header bytes)
func (cache *spaceUsed) SpaceUsedForPiecesLive(ctx context.Context) int64 {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.total
}

// SpaceUsedByNamespaceLive returns the current total space used for a specific
// satellite for all pieces (not including header bytes)
func (cache *spaceUsed) SpaceUsedByNamespaceLive(ctx context.Context, namespace string) int64 {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.totalByNamespace[namespace]
}

// UpdateCache updates the live used space totals
// with a pieceSize that was either created or deleted where the pieceSize is
// only the content size and does not include header bytes
func (cache *spaceUsed) UpdateCache(ctx context.Context, namespace string, pieceSize int64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.total += pieceSize
	cache.totalByNamespace[namespace] += pieceSize
}

// Recalculate iterates over all blobs on disk and recalculates
// the totals store in the cache
func (cache *spaceUsed) Recalculate(ctx context.Context, newtotalSpaceUsed int64, newtotalSpaceUsedByNamespace map[string]int64) error {
	spaceUsedWhenIterationStarted := cache

	var estimatedTotals int64
	estimatedTotals = estimate(newtotalSpaceUsed,
		spaceUsedWhenIterationStarted.total,
		cache.total,
	)

	var estimatedTotalsByNamespace map[string]int64
	for ns, newTotal := range newtotalSpaceUsedByNamespace {
		estimatedTotalsByNamespace[ns] = estimate(newTotal,
			spaceUsedWhenIterationStarted.total,
			cache.total,
		)
	}

	cache.mu.Lock()
	cache.total = estimatedTotals
	cache.totalByNamespace = estimatedTotalsByNamespace
	cache.mu.Unlock()
	return nil
}

func estimate(newSpaceUsedTotal, spaceUsedWhenIterationStarted, spaceUsedWhenIterationEnded int64) int64 {
	if newSpaceUsedTotal == spaceUsedWhenIterationEnded {
		return newSpaceUsedTotal
	}

	// If we missed writes/deletes while iterating, we will assume that half of those missed occurred before
	// the iteration and half occurred after. So here we add half of the delta to the result space used totals
	// from the iteration to account for those missed.
	spaceUsedDeltaDuringIteration := spaceUsedWhenIterationStarted - spaceUsedWhenIterationEnded
	return newSpaceUsedTotal + (spaceUsedDeltaDuringIteration / 2)
}
