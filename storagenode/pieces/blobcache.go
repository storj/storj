// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/storage"
)

// BlobsUsageCache is a blob storage with a cache for storing
// live values for current space used
type BlobsUsageCache struct {
	storage.Blobs

	mu    sync.Mutex
	cache spaceUsed
}

type spaceUsed struct {
	total            int64
	totalBySatellite map[string]int64
}

// NewBlobsUsageCache creates a new disk blob store with a space used cache
func NewBlobsUsageCache(blob storage.Blobs) *BlobsUsageCache {
	return &BlobsUsageCache{
		Blobs: blob,
		cache: spaceUsed{
			totalBySatellite: map[string]int64{},
		},
	}
}

type CacheService struct {
	log             *zap.Logger
	blobsUsageCache *BlobsUsageCache
	store           *Store
	loop            sync2.Cycle
}

// NewService creates a new cache service that updates the space usage cache on an interval
func NewService(log *zap.Logger, blobsUsageCache *BlobsUsageCache, pieces *Store, interval time.Duration) *CacheService {
	return &CacheService{
		log:             log,
		blobsUsageCache: blobsUsageCache,
		store:           pieces,
		loop:            *sync2.NewCycle(interval),
	}
}

func (service *CacheService) Run(ctx context.Context) (err error) {
	return service.loop.Run(ctx, func(ctx context.Context) error {

		totalSpaceUsed, totalSpaceUsedBySatellite, err := service.store.SpaceUsedTotalAndBySatellite(ctx)
		if err != nil {
			service.log.Error("error getting current space used calculation: ", zap.Error(err))
		}
		service.blobsUsageCache.recalculate(ctx, totalSpaceUsed, totalSpaceUsedBySatellite)
		if err != nil {
			service.log.Error("error during recalculating space usage cache: ", zap.Error(err))
		}
		service.log.Debug("recalculating...", zap.Int64("new total:", service.blobsUsageCache.cache.total))
		return err
	})
}

// Total satisfies the pieces interface
func (blobs *BlobsUsageCache) Total() int64 {
	return blobs.cache.total
}

// SpaceUsedBySatelliteLive returns the current total space used for a specific
// satellite for all pieces (not including header bytes)
func (blobs *BlobsUsageCache) SpaceUsedBySatelliteLive(ctx context.Context, satelliteID string) int64 {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	return blobs.cache.totalBySatellite[satelliteID]
}

// SpaceUsedForPiecesLive returns the current total used space for
//// all pieces content (not including header bytes)
func (blobs *BlobsUsageCache) SpaceUsedForPiecesLive(ctx context.Context) int64 {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	return blobs.cache.total
}

func (blobs *BlobsUsageCache) update(ctx context.Context, satelliteID string, pieceContentSize int64) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	blobs.cache.total += pieceContentSize
	blobs.cache.totalBySatellite[satelliteID] += pieceContentSize
}

// Close satisfies the pieces interface
func (blobs *BlobsUsageCache) Close() error {
	return nil
}

func (blobs *BlobsUsageCache) recalculate(ctx context.Context, newTotalSpaceUsed int64, newTotalSpaceUsedByNamespace map[string]int64) error {
	spaceUsedWhenIterationStarted := blobs.cache

	var estimatedTotals int64
	estimatedTotals = estimate(newTotalSpaceUsed,
		spaceUsedWhenIterationStarted.total,
		blobs.cache.total,
	)

	var estimatedTotalsBySatellite = map[string]int64{}
	for sa, newTotal := range newTotalSpaceUsedByNamespace {
		estimatedTotalsBySatellite[sa] = estimate(newTotal,
			spaceUsedWhenIterationStarted.total,
			blobs.cache.total,
		)
	}

	blobs.mu.Lock()
	blobs.cache.total = estimatedTotals
	blobs.cache.totalBySatellite = estimatedTotalsBySatellite
	blobs.mu.Unlock()
	return nil
}

func estimate(newSpaceUsedTotal, spaceUsedWhenIterationStarted, spaceUsedWhenIterationEnded int64) int64 {
	if newSpaceUsedTotal == spaceUsedWhenIterationEnded {
		return newSpaceUsedTotal
	}
	spaceUsedDeltaDuringIteration := spaceUsedWhenIterationEnded - spaceUsedWhenIterationStarted
	return newSpaceUsedTotal + (spaceUsedDeltaDuringIteration / 2)
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

	blobs.update(ctx, string(blobRef.Namespace), pieceContentSize)
	return nil
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
// created then it calls the blobWriter to complete the upload.
func (blob *blobCacheWriter) Commit(ctx context.Context) error {
	// get the size written we commit that way this
	// value will only include the piece content size and not
	// the header bytes
	size, err := blob.BlobWriter.Size()
	if err != nil {
		return Error.Wrap(err)
	}
	blob.usageCache.update(ctx, blob.namespace, size)

	err = blob.BlobWriter.Commit(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}
