// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// CacheService updates the space used cache
type CacheService struct {
	log        *zap.Logger
	usageCache *BlobsUsageCache
	store      *Store
	loop       sync2.Cycle
}

// NewService creates a new cache service that updates the space usage cache on startup and syncs the cache values to
// persistent storage on an interval
func NewService(log *zap.Logger, usageCache *BlobsUsageCache, pieces *Store, interval time.Duration) *CacheService {
	return &CacheService{
		log:        log,
		usageCache: usageCache,
		store:      pieces,
		loop:       *sync2.NewCycle(interval),
	}
}

// Run recalculates the space used cache once and also runs a loop to sync the space used cache
// to persistent storage on an interval
func (service *CacheService) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	totalAtStart := service.usageCache.copyCacheTotals()

	// recalculate the cache once
	newTotal, newTotalBySatellite, err := service.store.SpaceUsedTotalAndBySatellite(ctx)
	if err != nil {
		service.log.Error("error getting current space used calculation: ", zap.Error(err))
	}
	if err = service.usageCache.Recalculate(ctx, newTotal,
		totalAtStart.totalSpaceUsed,
		newTotalBySatellite,
		totalAtStart.totalSpaceUsedBySatellite,
	); err != nil {
		service.log.Error("error during recalculating space usage cache: ", zap.Error(err))
	}

	if err = service.store.spaceUsedDB.Init(ctx); err != nil {
		service.log.Error("error during init space usage db: ", zap.Error(err))
	}

	return service.loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		// on a loop sync the cache values to the db so that we have the them saved
		// in the case that the storagenode restarts
		if err := service.PersistCacheTotals(ctx); err != nil {
			service.log.Error("error persisting cache totals to the database: ", zap.Error(err))
		}
		return err
	})
}

// PersistCacheTotals saves the current totals of the space used cache to the database
// so that if the storagenode restarts it can retrieve the latest space used
// values without needing to recalculate since that could take a long time
func (service *CacheService) PersistCacheTotals(ctx context.Context) error {
	cache := service.usageCache
	if err := service.store.spaceUsedDB.UpdateTotal(ctx, cache.totalSpaceUsed); err != nil {
		return err
	}
	if err := service.store.spaceUsedDB.UpdateTotalsForAllSatellites(ctx, cache.totalSpaceUsedBySatellite); err != nil {
		return err
	}
	return nil
}

// Init initializes the space used cache with the most recent values that were stored persistently
func (service *CacheService) Init(ctx context.Context) (err error) {
	total, err := service.store.spaceUsedDB.GetTotal(ctx)
	if err != nil {
		service.log.Error("CacheServiceInit error during initializing space usage cache GetTotal:", zap.Error(err))
		return err
	}

	totalBySatellite, err := service.store.spaceUsedDB.GetTotalsForAllSatellites(ctx)
	if err != nil {
		service.log.Error("CacheServiceInit error during initializing space usage cache GetTotalsForAllSatellites:", zap.Error(err))
		return err
	}

	service.usageCache.init(total, totalBySatellite)
	return nil
}

// Close closes the loop
func (service *CacheService) Close() (err error) {
	service.loop.Close()
	return nil
}

// BlobsUsageCache is a blob storage with a cache for storing
// totals of current space used
type BlobsUsageCache struct {
	storage.Blobs

	mu                        sync.Mutex
	totalSpaceUsed            int64
	totalSpaceUsedBySatellite map[storj.NodeID]int64
}

// NewBlobsUsageCache creates a new disk blob store with a space used cache
func NewBlobsUsageCache(blob storage.Blobs) *BlobsUsageCache {
	return &BlobsUsageCache{
		Blobs:                     blob,
		totalSpaceUsedBySatellite: map[storj.NodeID]int64{},
	}
}

// NewBlobsUsageCacheTest creates a new disk blob store with a space used cache
func NewBlobsUsageCacheTest(blob storage.Blobs, total int64, totalSpaceUsedBySatellite map[storj.NodeID]int64) *BlobsUsageCache {
	return &BlobsUsageCache{
		Blobs:                     blob,
		totalSpaceUsed:            total,
		totalSpaceUsedBySatellite: totalSpaceUsedBySatellite,
	}
}

func (blobs *BlobsUsageCache) init(total int64, totalBySatellite map[storj.NodeID]int64) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	blobs.totalSpaceUsed = total
	blobs.totalSpaceUsedBySatellite = totalBySatellite
}

// SpaceUsedBySatellite returns the current total space used for a specific
// satellite for all pieces (not including header bytes)
func (blobs *BlobsUsageCache) SpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (int64, error) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	return blobs.totalSpaceUsedBySatellite[satelliteID], nil
}

// SpaceUsedForPieces returns the current total used space for
//// all pieces content (not including header bytes)
func (blobs *BlobsUsageCache) SpaceUsedForPieces(ctx context.Context) (int64, error) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	return blobs.totalSpaceUsed, nil
}

// Delete gets the size of the piece that is going to be deleted then deletes it and
// updates the space used cache accordingly
func (blobs *BlobsUsageCache) Delete(ctx context.Context, blobRef storage.BlobRef) error {
	blobInfo, err := blobs.Stat(ctx, blobRef)
	if err != nil {
		return err
	}
	pieceAccess, err := newStoredPieceAccess(nil, blobInfo)
	if err != nil {
		return err
	}
	pieceContentSize, err := pieceAccess.ContentSize(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	if err := blobs.Blobs.Delete(ctx, blobRef); err != nil {
		return Error.Wrap(err)
	}

	satelliteID := storj.NodeID{}
	copy(satelliteID[:], blobRef.Namespace)
	blobs.Update(ctx, satelliteID, -pieceContentSize)
	return nil
}

// Update updates the cache totals with the piece content size
func (blobs *BlobsUsageCache) Update(ctx context.Context, satelliteID storj.NodeID, pieceContentSize int64) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	blobs.totalSpaceUsed += pieceContentSize
	blobs.totalSpaceUsedBySatellite[satelliteID] += pieceContentSize
}

func (blobs *BlobsUsageCache) copyCacheTotals() BlobsUsageCache {
	var copyMap = map[storj.NodeID]int64{}
	for k, v := range blobs.totalSpaceUsedBySatellite {
		copyMap[k] = v
	}
	return BlobsUsageCache{
		totalSpaceUsed:            blobs.totalSpaceUsed,
		totalSpaceUsedBySatellite: copyMap,
	}
}

// Recalculate estimates new totals for the space used cache. In order to get new totals for the
// space used cache, we had to iterate over all the pieces on disk. Since that can potentially take
// a long time, here we need to check if we missed any additions/deletions while we were iterating and
// estimate how many bytes missed then add those to the space used result of iteration.
func (blobs *BlobsUsageCache) Recalculate(ctx context.Context, newTotal, totalAtIterationStart int64, newTotalBySatellite, totalBySatelliteAtIterationStart map[storj.NodeID]int64) error {
	blobs.mu.Lock()
	totalsAtIterationEnd := blobs.copyCacheTotals()
	blobs.mu.Unlock()

	estimatedTotals := estimate(newTotal,
		totalAtIterationStart,
		totalsAtIterationEnd.totalSpaceUsed,
	)

	var estimatedTotalsBySatellite = map[storj.NodeID]int64{}
	for ID, newTotal := range newTotalBySatellite {
		estimatedNewTotal := estimate(newTotal,
			totalBySatelliteAtIterationStart[ID],
			totalsAtIterationEnd.totalSpaceUsedBySatellite[ID],
		)
		// if the estimatedNewTotal is zero then there is no data stored
		// for this satelliteID so don't add it to the cache
		if estimatedNewTotal == 0 {
			continue
		}
		estimatedTotalsBySatellite[ID] = estimatedNewTotal
	}

	// find any saIDs that are in totalsAtIterationEnd but not in newTotalSpaceUsedBySatellite
	missedWhenIterationEnded := getMissed(totalsAtIterationEnd.totalSpaceUsedBySatellite,
		newTotalBySatellite,
	)
	if len(missedWhenIterationEnded) > 0 {
		for ID := range missedWhenIterationEnded {
			estimatedNewTotal := estimate(0,
				totalBySatelliteAtIterationStart[ID],
				totalsAtIterationEnd.totalSpaceUsedBySatellite[ID],
			)
			if estimatedNewTotal == 0 {
				continue
			}
			estimatedTotalsBySatellite[ID] = estimatedNewTotal
		}
	}

	blobs.mu.Lock()
	blobs.totalSpaceUsed = estimatedTotals
	blobs.totalSpaceUsedBySatellite = estimatedTotalsBySatellite
	blobs.mu.Unlock()
	return nil
}

func estimate(newSpaceUsedTotal, totalAtIterationStart, totalAtIterationEnd int64) int64 {
	if newSpaceUsedTotal == totalAtIterationEnd {
		return newSpaceUsedTotal
	}

	// If we missed writes/deletes while iterating, we will assume that half of those missed occurred before
	// the iteration and half occurred after. So here we add half of the delta to the result space used totals
	// from the iteration to account for those missed.
	spaceUsedDeltaDuringIteration := totalAtIterationEnd - totalAtIterationStart
	estimatedTotal := newSpaceUsedTotal + (spaceUsedDeltaDuringIteration / 2)
	if estimatedTotal < 0 {
		return 0
	}
	return estimatedTotal
}

func getMissed(endTotals, newTotals map[storj.NodeID]int64) map[storj.NodeID]int64 {
	var missed = map[storj.NodeID]int64{}
	for id, total := range endTotals {
		if _, ok := newTotals[id]; !ok {
			missed[id] = total
		}
	}
	return missed
}

// Close satisfies the pieces interface
func (blobs *BlobsUsageCache) Close() error {
	return nil
}

// TestCreateV0 creates a new V0 blob that can be written. This is only appropriate in test situations.
func (blobs *BlobsUsageCache) TestCreateV0(ctx context.Context, ref storage.BlobRef) (_ storage.BlobWriter, err error) {
	fStore := blobs.Blobs.(interface {
		TestCreateV0(ctx context.Context, ref storage.BlobRef) (_ storage.BlobWriter, err error)
	})
	return fStore.TestCreateV0(ctx, ref)
}
