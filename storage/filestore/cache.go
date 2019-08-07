// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"storj.io/storj/storage"
)

// StoreUsageCache is a blob storage with a cache for storing
// live values for current space used
type StoreUsageCache struct {
	storage.Blobs

	cacheMutex sync.Mutex
	cache      spaceUsed
}

type spaceUsed struct {
	total            int64
	totalByNamespace map[string]int64
}

// NewWithCache creates a new disk blob store with a cache in the specified directory
func NewWithCache(dir *Dir, log *zap.Logger) *StoreUsageCache {
	return &StoreUsageCache{
		Blobs: &Store{dir: dir, log: log},
	}
}

// InitCache initializes the cache with total values of current space usage
func (store *StoreUsageCache) InitCache(ctx context.Context) error {
	newtotalSpaceUsed, newtotalSpaceUsedByNamespace, err := store.SpaceUsedTotalAndByNamespace(ctx)
	if err != nil {
		return err
	}
	store.cacheMutex.Lock()
	store.cache.total = newtotalSpaceUsed
	store.cache.totalByNamespace = newtotalSpaceUsedByNamespace
	store.cacheMutex.Unlock()
	return nil
}

// SpaceUsedForPiecesLive returns the current total used space for
// all pieces content (not including header bytes)
func (store *StoreUsageCache) SpaceUsedForPiecesLive(ctx context.Context) int64 {
	store.cacheMutex.Lock()
	defer store.cacheMutex.Unlock()
	return store.cache.total
}

// SpaceUsedByNamespaceLive returns the current total space used for a specific
// satellite for all pieces (not including header bytes)
func (store *StoreUsageCache) SpaceUsedByNamespaceLive(ctx context.Context, namespace string) int64 {
	store.cacheMutex.Lock()
	defer store.cacheMutex.Unlock()
	return store.cache.totalByNamespace[namespace]
}

// UpdateCache updates the live used space totals
// with a pieceSize that was either created or deleted where the pieceSize is
// only the content size and does not include header bytes
func (store *StoreUsageCache) UpdateCache(ctx context.Context, namespace string, pieceSize int64) {
	store.cacheMutex.Lock()
	defer store.cacheMutex.Unlock()
	store.cache.total += pieceSize
	store.cache.totalByNamespace[namespace] += pieceSize
}

// RecalculateCache iterates over all blobs on disk and recalculates
// the totals store in the cache
func (store *StoreUsageCache) RecalculateCache(ctx context.Context) error {
	spaceUsedWhenIterationStarted := store.cache

	newtotalSpaceUsed, newtotalSpaceUsedByNamespace, err := store.SpaceUsedTotalAndByNamespace(ctx)
	if err != nil {
		return err
	}

	var estimatedTotals int64
	estimatedTotals = estimate(newtotalSpaceUsed,
		spaceUsedWhenIterationStarted.total,
		store.cache.total,
	)

	var estimatedTotalsByNamespace map[string]int64
	for ns, newTotal := range newtotalSpaceUsedByNamespace {
		estimatedTotalsByNamespace[ns] = estimate(newTotal,
			spaceUsedWhenIterationStarted.total,
			store.cache.total,
		)
	}

	store.cacheMutex.Lock()
	store.cache.total = estimatedTotals
	store.cache.totalByNamespace = estimatedTotalsByNamespace
	store.cacheMutex.Unlock()
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

// Close closed the store
func (store *StoreUsageCache) Close() error {
	return nil
}
