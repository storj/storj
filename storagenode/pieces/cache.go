// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storage"
)

// CacheService updates the space used cache
//
// architecture: Chore
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
	newTrashTotal, err := service.store.SpaceUsedForTrash(ctx)
	if err != nil {
		service.log.Error("error getting current space for trash: ", zap.Error(err))
	}
	service.usageCache.Recalculate(ctx, newTotal,
		totalAtStart.spaceUsedForPieces,
		newTotalBySatellite,
		totalAtStart.spaceUsedBySatellite,
		newTrashTotal,
		totalAtStart.spaceUsedForTrash,
	)

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
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if err := service.store.spaceUsedDB.UpdatePieceTotal(ctx, cache.spaceUsedForPieces); err != nil {
		return err
	}
	if err := service.store.spaceUsedDB.UpdatePieceTotalsForAllSatellites(ctx, cache.spaceUsedBySatellite); err != nil {
		return err
	}
	if err := service.store.spaceUsedDB.UpdateTrashTotal(ctx, cache.spaceUsedForTrash); err != nil {
		return err
	}
	return nil
}

// Init initializes the space used cache with the most recent values that were stored persistently
func (service *CacheService) Init(ctx context.Context) (err error) {
	total, err := service.store.spaceUsedDB.GetPieceTotal(ctx)
	if err != nil {
		service.log.Error("CacheServiceInit error during initializing space usage cache GetTotal:", zap.Error(err))
		return err
	}

	totalBySatellite, err := service.store.spaceUsedDB.GetPieceTotalsForAllSatellites(ctx)
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
//
// architecture: Database
type BlobsUsageCache struct {
	storage.Blobs

	mu                   sync.Mutex
	spaceUsedForPieces   int64
	spaceUsedForTrash    int64
	spaceUsedBySatellite map[storj.NodeID]int64
}

// NewBlobsUsageCache creates a new disk blob store with a space used cache
func NewBlobsUsageCache(blob storage.Blobs) *BlobsUsageCache {
	return &BlobsUsageCache{
		Blobs:                blob,
		spaceUsedBySatellite: map[storj.NodeID]int64{},
	}
}

// NewBlobsUsageCacheTest creates a new disk blob store with a space used cache
func NewBlobsUsageCacheTest(blob storage.Blobs, piecesTotal, trashTotal int64, spaceUsedBySatellite map[storj.NodeID]int64) *BlobsUsageCache {
	return &BlobsUsageCache{
		Blobs:                blob,
		spaceUsedForPieces:   piecesTotal,
		spaceUsedForTrash:    trashTotal,
		spaceUsedBySatellite: spaceUsedBySatellite,
	}
}

func (blobs *BlobsUsageCache) init(total int64, totalBySatellite map[storj.NodeID]int64) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	blobs.spaceUsedForPieces = total
	blobs.spaceUsedBySatellite = totalBySatellite
}

// SpaceUsedBySatellite returns the current total space used for a specific
// satellite for all pieces (not including header bytes)
func (blobs *BlobsUsageCache) SpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (int64, error) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	return blobs.spaceUsedBySatellite[satelliteID], nil
}

// SpaceUsedForPieces returns the current total used space for
//// all pieces content (not including header bytes)
func (blobs *BlobsUsageCache) SpaceUsedForPieces(ctx context.Context) (int64, error) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	return blobs.spaceUsedForPieces, nil
}

// SpaceUsedForTrash returns the current total used space for the trash dir
func (blobs *BlobsUsageCache) SpaceUsedForTrash(ctx context.Context) (int64, error) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	return blobs.spaceUsedForTrash, nil
}

// Delete gets the size of the piece that is going to be deleted then deletes it and
// updates the space used cache accordingly
func (blobs *BlobsUsageCache) Delete(ctx context.Context, blobRef storage.BlobRef) error {
	_, pieceContentSize, err := blobs.pieceContentSize(ctx, blobRef)
	if err != nil {
		return Error.Wrap(err)
	}

	if err := blobs.Blobs.Delete(ctx, blobRef); err != nil {
		return Error.Wrap(err)
	}

	satelliteID, err := storj.NodeIDFromBytes(blobRef.Namespace)
	if err != nil {
		return err
	}
	blobs.Update(ctx, satelliteID, -pieceContentSize, 0)
	return nil
}

func (blobs *BlobsUsageCache) pieceContentSize(ctx context.Context, blobRef storage.BlobRef) (size int64, contentSize int64, err error) {
	blobInfo, err := blobs.Stat(ctx, blobRef)
	if err != nil {
		return 0, 0, err
	}
	pieceAccess, err := newStoredPieceAccess(nil, blobInfo)
	if err != nil {
		return 0, 0, err
	}
	return pieceAccess.Size(ctx)
}

// Update updates the cache totals with the piece content size
func (blobs *BlobsUsageCache) Update(ctx context.Context, satelliteID storj.NodeID, piecesDelta, trashDelta int64) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	blobs.spaceUsedForPieces += piecesDelta
	blobs.spaceUsedBySatellite[satelliteID] += piecesDelta
	blobs.spaceUsedForTrash += trashDelta
}

// Trash moves the ref to the trash and updates the cache
func (blobs *BlobsUsageCache) Trash(ctx context.Context, blobRef storage.BlobRef) error {
	size, pieceContentSize, err := blobs.pieceContentSize(ctx, blobRef)
	if err != nil {
		return Error.Wrap(err)
	}

	err = blobs.Blobs.Trash(ctx, blobRef)
	if err != nil {
		return Error.Wrap(err)
	}

	satelliteID, err := storj.NodeIDFromBytes(blobRef.Namespace)
	if err != nil {
		return Error.Wrap(err)
	}

	blobs.Update(ctx, satelliteID, -pieceContentSize, size)
	return nil
}

// EmptyTrash empties the trash and updates the cache
func (blobs *BlobsUsageCache) EmptyTrash(ctx context.Context, namespace []byte, trashedBefore time.Time) (int64, [][]byte, error) {
	satelliteID, err := storj.NodeIDFromBytes(namespace)
	if err != nil {
		return 0, nil, err
	}

	bytesEmptied, keys, err := blobs.Blobs.EmptyTrash(ctx, namespace, trashedBefore)
	if err != nil {
		return 0, nil, err
	}

	blobs.Update(ctx, satelliteID, 0, -bytesEmptied)

	return bytesEmptied, keys, nil
}

// RestoreTrash restores the trash for the namespace and updates the cache
func (blobs *BlobsUsageCache) RestoreTrash(ctx context.Context, namespace []byte) ([][]byte, error) {
	satelliteID, err := storj.NodeIDFromBytes(namespace)
	if err != nil {
		return nil, err
	}

	keysRestored, err := blobs.Blobs.RestoreTrash(ctx, namespace)
	if err != nil {
		return nil, err
	}

	for _, key := range keysRestored {
		size, contentSize, sizeErr := blobs.pieceContentSize(ctx, storage.BlobRef{
			Key:       key,
			Namespace: namespace,
		})
		if sizeErr != nil {
			err = errs.Combine(err, sizeErr)
			continue
		}
		blobs.Update(ctx, satelliteID, contentSize, -size)
	}

	return keysRestored, err
}

func (blobs *BlobsUsageCache) copyCacheTotals() BlobsUsageCache {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	var copyMap = map[storj.NodeID]int64{}
	for k, v := range blobs.spaceUsedBySatellite {
		copyMap[k] = v
	}
	return BlobsUsageCache{
		spaceUsedForPieces:   blobs.spaceUsedForPieces,
		spaceUsedForTrash:    blobs.spaceUsedForTrash,
		spaceUsedBySatellite: copyMap,
	}
}

// Recalculate estimates new totals for the space used cache. In order to get new totals for the
// space used cache, we had to iterate over all the pieces on disk. Since that can potentially take
// a long time, here we need to check if we missed any additions/deletions while we were iterating and
// estimate how many bytes missed then add those to the space used result of iteration.
func (blobs *BlobsUsageCache) Recalculate(ctx context.Context, newTotal, totalAtIterationStart int64, newTotalBySatellite,
	totalBySatelliteAtIterationStart map[storj.NodeID]int64, newTrashTotal, trashTotalAtIterationStart int64) {

	totalsAtIterationEnd := blobs.copyCacheTotals()

	estimatedTotals := estimate(newTotal,
		totalAtIterationStart,
		totalsAtIterationEnd.spaceUsedForPieces,
	)

	estimatedTrash := estimate(newTrashTotal,
		trashTotalAtIterationStart,
		totalsAtIterationEnd.spaceUsedForTrash)

	var estimatedTotalsBySatellite = map[storj.NodeID]int64{}
	for ID, newTotal := range newTotalBySatellite {
		estimatedNewTotal := estimate(newTotal,
			totalBySatelliteAtIterationStart[ID],
			totalsAtIterationEnd.spaceUsedBySatellite[ID],
		)
		// if the estimatedNewTotal is zero then there is no data stored
		// for this satelliteID so don't add it to the cache
		if estimatedNewTotal == 0 {
			continue
		}
		estimatedTotalsBySatellite[ID] = estimatedNewTotal
	}

	// find any saIDs that are in totalsAtIterationEnd but not in newTotalSpaceUsedBySatellite
	missedWhenIterationEnded := getMissed(totalsAtIterationEnd.spaceUsedBySatellite,
		newTotalBySatellite,
	)
	if len(missedWhenIterationEnded) > 0 {
		for ID := range missedWhenIterationEnded {
			estimatedNewTotal := estimate(0,
				totalBySatelliteAtIterationStart[ID],
				totalsAtIterationEnd.spaceUsedBySatellite[ID],
			)
			if estimatedNewTotal == 0 {
				continue
			}
			estimatedTotalsBySatellite[ID] = estimatedNewTotal
		}
	}

	blobs.mu.Lock()
	blobs.spaceUsedForPieces = estimatedTotals
	blobs.spaceUsedForTrash = estimatedTrash
	blobs.spaceUsedBySatellite = estimatedTotalsBySatellite
	blobs.mu.Unlock()
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
