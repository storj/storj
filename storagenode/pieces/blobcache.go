// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"storj.io/storj/pkg/storj"
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
	totalBySatellite map[storj.NodeID]int64
}

// NewBlobsUsageCache creates a new disk blob store with a space used cache
func NewBlobsUsageCache(blob storage.Blobs) *BlobsUsageCache {
	return &BlobsUsageCache{
		Blobs: blob,
		cache: spaceUsed{
			totalBySatellite: map[storj.NodeID]int64{},
		},
	}
}

// CacheService updates the space used cache
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

// Run runs the cache service loop which recalculates and updates the space used cache
func (service *CacheService) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// recalculate on start up
	total, totalBySatellite, err := service.store.SpaceUsedTotalAndBySatellite(ctx)
	if err != nil {
		service.log.Error("error getting current space used calculation: ", zap.Error(err))
	}
	if err = service.blobsUsageCache.recalculate(ctx, total, totalBySatellite); err != nil {
		service.log.Error("error during recalculating space usage cache: ", zap.Error(err))
	}

	service.log.Debug("CacheService Run:", zap.Int64("new total:", service.blobsUsageCache.cache.total))
	for saID, t := range totalBySatellite {
		service.log.Debug("CacheService Run:", zap.String("saID", saID.String()), zap.Int64("new sa total:", t))
	}

	return service.loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		// on loop sync cache to db in case of restart
		if err := service.persistCacheTotals(ctx); err != nil {
			service.log.Error("error during initializing space usage cache, saveCache: ", zap.Error(err))
		}
		return err
	})
}

func (service *CacheService) persistCacheTotals(ctx context.Context) error {
	cache := service.blobsUsageCache.cache
	if err := service.store.spaceUsedDB.UpdateTotal(ctx, cache.total); err != nil {
		return err
	}
	if err := service.store.spaceUsedDB.UpdateTotalsForAllSatellites(ctx, cache.totalBySatellite); err != nil {
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
	service.log.Debug("CacheServiceInit initializing cache GetTotal", zap.Int64("new total:", total))

	totalBySatellites, err := service.store.spaceUsedDB.GetTotalsForAllSatellites(ctx)
	if err != nil {
		service.log.Error("CacheServiceInit error during initializing space usage cache GetTotalsForAllSatellites:", zap.Error(err))
		return err
	}
	for _, saTotal := range totalBySatellites {
		service.log.Debug("CacheServiceInit initializing cache SA total", zap.Int64("new totalBySA:", saTotal))
	}

	service.blobsUsageCache.init(total, totalBySatellites)
	return nil
}

// Close closes the loop
func (service *CacheService) Close() (err error) {
	service.loop.Close()
	return nil
}

func (blobs *BlobsUsageCache) init(total int64, totalBySatellite map[storj.NodeID]int64) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	blobs.cache.total = total
	blobs.cache.totalBySatellite = totalBySatellite
}

// SpaceUsedBySatellite returns the current total space used for a specific
// satellite for all pieces (not including header bytes)
func (blobs *BlobsUsageCache) SpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (int64, error) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	return blobs.cache.totalBySatellite[satelliteID], nil
}

// SpaceUsedForPieces returns the current total used space for
//// all pieces content (not including header bytes)
func (blobs *BlobsUsageCache) SpaceUsedForPieces(ctx context.Context) (int64, error) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	return blobs.cache.total, nil
}

func (blobs *BlobsUsageCache) update(ctx context.Context, satelliteID storj.NodeID, pieceContentSize int64) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	blobs.cache.total += pieceContentSize
	blobs.cache.totalBySatellite[satelliteID] += pieceContentSize
}

// Close satisfies the pieces interface
func (blobs *BlobsUsageCache) Close() error {
	return nil
}

func (blobs *BlobsUsageCache) recalculate(ctx context.Context, newTotalSpaceUsed int64, newTotalSpaceUsedByNamespace map[storj.NodeID]int64) error {
	spaceUsedWhenIterationStarted := blobs.cache

	var estimatedTotals int64
	estimatedTotals = estimate(newTotalSpaceUsed,
		spaceUsedWhenIterationStarted.total,
		blobs.cache.total,
	)

	var estimatedTotalsBySatellite = map[storj.NodeID]int64{}
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

	satelliteID := storj.NodeID{}
	satelliteID.Scan(blobRef.Namespace)
	blobs.update(ctx, satelliteID, pieceContentSize)
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
	satelliteID := storj.NodeID{}
	satelliteID.Scan(blob.namespace)
	blob.usageCache.update(ctx, satelliteID, size)

	err = blob.BlobWriter.Commit(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}
