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

	spaceUsedWhenIterationStarted := service.usageCache

	// recalculate the cache once
	newTotalSpaceUsed, newTotalSpaceUsedBySatellite, err := service.store.SpaceUsedTotalAndBySatellite(ctx)
	if err != nil {
		service.log.Error("error getting current space used calculation: ", zap.Error(err))
	}
	if err = service.usageCache.Recalculate(ctx, newTotalSpaceUsed, newTotalSpaceUsedBySatellite, *spaceUsedWhenIterationStarted); err != nil {
		service.log.Error("error during recalculating space usage cache: ", zap.Error(err))
	}

	return service.loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		// on a loop sync the cache values to the db so that we have the them saved
		// in the case that the storagenode restarts
		if err := service.persistCacheTotals(ctx); err != nil {
			service.log.Error("error persisting cache totals to the database: ", zap.Error(err))
		}
		return err
	})
}

// persistCacheTotals saves the current totals of the space used cache to the database
// so that if the storagenode restarts it can retrieve the latest space used
// values without needing to recalculate since that could take a long time
func (service *CacheService) persistCacheTotals(ctx context.Context) error {
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

// Create returns a blobWriter that knows which namespace/satellite its writing the piece to
// and also has access to the space used cache to update when finished writing the new piece
func (blobs *BlobsUsageCache) Create(ctx context.Context, ref storage.BlobRef, size int64) (_ storage.BlobWriter, err error) {
	blobWriter, err := blobs.Blobs.Create(ctx, ref, size)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &blobCacheWriter{
		BlobWriter: blobWriter,
		usageCache: blobs,
		namespace:  string(ref.Namespace),
	}, nil
}

// Delete gets the size of the piece that is going to be deleted then deletes it and
// updates the space used cache accordingly
func (blobs *BlobsUsageCache) Delete(ctx context.Context, blobRef storage.BlobRef) error {
	blobInfo, err := blobs.Stat(ctx, blobRef)
	if err != nil {
		return err
	}
	// calling with nil for store since we don't need it to
	// get content size
	pieceAccess, err := newStoredPieceAccess(nil, blobInfo)
	if err != nil {
		return err
	}
	pieceContentSize, err := pieceAccess.ContentSize(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	err = blobs.Blobs.Delete(ctx, blobRef)
	if err != nil {
		return Error.Wrap(err)
	}

	satelliteID := storj.NodeID{}
	satelliteID.Scan(blobRef.Namespace)
	blobs.update(ctx, satelliteID, -pieceContentSize)
	return nil
}

func (blobs *BlobsUsageCache) update(ctx context.Context, satelliteID storj.NodeID, pieceContentSize int64) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	blobs.totalSpaceUsed += pieceContentSize
	blobs.totalSpaceUsedBySatellite[satelliteID] += pieceContentSize
}

// Recalculate estimates new totals for the space used cache. In order to get new totals for the
// space used cache, we had to iterate over all the pieces on disk. Since that can potentially take
// a long time, here we need to check if we missed any additions/deletions while we were iterating and
// estimate how many bytes missed then add those to the space used result of iteration.
func (blobs *BlobsUsageCache) Recalculate(ctx context.Context, newTotalSpaceUsed int64, newTotalSpaceUsedBySatellite map[storj.NodeID]int64, spaceUsedWhenIterationStarted BlobsUsageCache) error {
	spaceUsedWhenIterationEnded := blobs

	var estimatedTotals int64
	estimatedTotals = estimate(newTotalSpaceUsed,
		spaceUsedWhenIterationStarted.totalSpaceUsed,
		spaceUsedWhenIterationEnded.totalSpaceUsed,
	)

	var estimatedTotalsBySatellite = map[storj.NodeID]int64{}
	for ID, newTotal := range newTotalSpaceUsedBySatellite {
		estimatedNewTotal := estimate(newTotal,
			spaceUsedWhenIterationStarted.totalSpaceUsed,
			spaceUsedWhenIterationEnded.totalSpaceUsed,
		)

		if estimatedNewTotal == 0 {
			continue
		}
		estimatedTotalsBySatellite[ID] = estimatedNewTotal
	}

	blobs.mu.Lock()
	blobs.totalSpaceUsed = estimatedTotals
	blobs.totalSpaceUsedBySatellite = estimatedTotalsBySatellite
	blobs.mu.Unlock()
	return nil
}

func estimate(newSpaceUsedTotal, spaceUsedWhenIterationStarted, spaceUsedWhenIterationEnded int64) int64 {
	if newSpaceUsedTotal == spaceUsedWhenIterationEnded {
		return newSpaceUsedTotal
	}

	// If we missed writes/deletes while iterating, we will assume that half of those missed occurred before
	// the iteration and half occurred after. So here we add half of the delta to the result space used totals
	// from the iteration to account for those missed.
	spaceUsedDeltaDuringIteration := spaceUsedWhenIterationEnded - spaceUsedWhenIterationStarted
	return newSpaceUsedTotal + (spaceUsedDeltaDuringIteration / 2)
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

type blobCacheWriter struct {
	storage.BlobWriter
	usageCache *BlobsUsageCache
	namespace  string
}

// Commit updates the cache with the size of the new piece that was just
// created then it calls the blobWriter commit to complete the upload.
func (blob *blobCacheWriter) Commit(ctx context.Context) error {
	pieceContentSize, err := blob.BlobWriter.Size()
	if err != nil {
		return Error.Wrap(err)
	}
	satelliteID := storj.NodeID{}
	satelliteID.Scan(blob.namespace)
	blob.usageCache.update(ctx, satelliteID, pieceContentSize)

	if err := blob.BlobWriter.Commit(ctx); err != nil {
		return Error.Wrap(err)
	}

	return nil
}
