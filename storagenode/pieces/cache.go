// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// StoreWithCache has access to the pieces store and also keeps track of real time
// totals of space used for all pieces and for all pieces by satellite
type StoreWithCache struct {
	Store

	cacheMutex sync.RWMutex
	cache      spaceUsedCache
}

type spaceUsedCache struct {
	total            int64
	totalBySatellite map[storj.NodeID]int64
}

// NewStoreWithCache creates a new piece store with a cache for real time
// totals of space used for all pieces and for all pieces by satellite
func NewStoreWithCache(log *zap.Logger, blobs storage.Blobs, v0PieceInfo V0PieceInfoDB, expirationInfo PieceExpirationDB) *StoreWithCache {
	return &StoreWithCache{
		Store: Store{
			log:            log,
			blobs:          blobs,
			v0PieceInfo:    v0PieceInfo,
			expirationInfo: expirationInfo,
		},
	}
}

// InitCache walks through all the pieces on disk and sums their sizes to
// get initial values for the storage cache totals of space used
func (store *StoreWithCache) InitCache() error {
	newCacheTotals, err := store.spaceUsedForPiecesAndBySatellitesSlow()
	if err != nil {
		return err
	}

	store.cacheMutex.Lock()
	defer store.cacheMutex.Unlock()
	store.cache.total = newCacheTotals.total
	store.cache.totalBySatellite = newCacheTotals.totalBySatellite
	return nil
}

// spaceUsedForPiecesAndBySatellitesSlow iterates over all the pieces stored on disk
// and sums the bytes for all pieces (not including headers) currently stored. These
// new summed up values are used for the live in memory values.
// The size of each piece is measuered sequentially and writes/deletes still occur while iterating.
// Any writes/deletes that occur late in the process are likely to be missed and therefore the result of this
// method is an estimate.
func (store *StoreWithCache) spaceUsedForPiecesAndBySatellitesSlow() (spaceUsedCache, error) {
	satelliteIDs, err := store.getAllStoringSatellites(nil)
	if err != nil {
		return spaceUsedCache{}, err
	}

	var totalUsed int64
	totalsBySatellites := map[storj.NodeID]int64{}
	for _, satelliteID := range satelliteIDs {
		spaceUsed, err := store.SpaceUsedBySatellite(nil, satelliteID)
		if err != nil {
			return spaceUsedCache{}, err
		}
		totalsBySatellites[satelliteID] = spaceUsed
		totalUsed += spaceUsed
	}

	return spaceUsedCache{
		total:            totalUsed,
		totalBySatellite: totalsBySatellites,
	}, nil
}

// SpaceUsedForPiecesLive returns the current total used space for
// all pieces content (not including header bytes)
func (store *StoreWithCache) SpaceUsedForPiecesLive(ctx context.Context) int64 {
	store.cacheMutex.Lock()
	defer store.cacheMutex.Unlock()
	return store.cache.total
}

// SpaceUsedBySatelliteLive returns the current total space used for a specific
// satellite for all pieces (not including header bytes)
func (store *StoreWithCache) SpaceUsedBySatelliteLive(ctx context.Context, satelliteID storj.NodeID) int64 {
	store.cacheMutex.Lock()
	defer store.cacheMutex.Unlock()
	return store.cache.totalBySatellite[satelliteID]
}

// UpdateCache updates the live used space totals
// with a pieceSize that was either created or deleted where the pieceSize is
// only the content size and does not include header bytes
func (store *StoreWithCache) UpdateCache(ctx context.Context, satelliteID storj.NodeID, pieceSize int64) {
	store.cacheMutex.Lock()
	defer store.cacheMutex.Unlock()
	store.cache.total += pieceSize
	store.cache.totalBySatellite[satelliteID] += pieceSize
}

// RecalculateSpaceUsedCache sums up the bytes for all pieces (not including headers) currently stored.
// The live values for used space are initially calculated when the storagenode starts up, then
// incrememted/decremeted when pieces are created/deleted. In addition we want to occasionally recalculate
// the live values to confirm correctness. This method RecalculateSpaceUsedCache is responsible for doing that.
func (store *StoreWithCache) RecalculateSpaceUsedCache(ctx context.Context) error {
	store.cacheMutex.Lock()
	// Save the current live values before we start recalculating
	// so we can compare them to what we recalculate
	spaceUsedWhenIterationStarted := store.cache
	store.cacheMutex.Unlock()

	spaceUsedResultOfIteration, err := store.spaceUsedForPiecesAndBySatellitesSlow()
	if err != nil {
		return err
	}

	store.cacheMutex.Lock()
	defer store.cacheMutex.Unlock()
	// Since it might have taken a long time to iterate over all the pieces, here we need to check if
	// we missed any writes/deletes of pieces while we were iterating and estimate
	store.estimateAndSave(spaceUsedWhenIterationStarted, spaceUsedResultOfIteration)

	return nil
}

func (store *StoreWithCache) estimateAndSave(oldSpaceUsed, newSpaceUsed spaceUsedCache) {
	estimatedTotalsBySatellites := map[storj.NodeID]int64{}
	for satelliteID, newSpaceUsedTotal := range newSpaceUsed.totalBySatellite {
		estimatedTotalsBySatellites[satelliteID] = estimateRecalculation(newSpaceUsedTotal,
			oldSpaceUsed.totalBySatellite[satelliteID],
			store.cache.totalBySatellite[satelliteID],
		)
	}

	store.cache.totalBySatellite = estimatedTotalsBySatellites
	store.cache.total = estimateRecalculation(newSpaceUsed.total,
		oldSpaceUsed.total,
		store.cache.total,
	)
}

func estimateRecalculation(newSpaceUsedTotal, spaceUsedWhenIterationStarted, spaceUsedWhenIterationEnded int64) int64 {
	if newSpaceUsedTotal == spaceUsedWhenIterationEnded {
		return newSpaceUsedTotal
	}

	// If we missed writes/deletes while iterating, we will assume that half of those missed occurred before
	// the iteration and half occurred after. So here we add half of the delta to the result space used totals
	// from the iteration to account for those missed.
	spaceUsedDeltaDuringIteration := spaceUsedWhenIterationStarted - spaceUsedWhenIterationEnded
	return newSpaceUsedTotal + (spaceUsedDeltaDuringIteration / 2)
}
