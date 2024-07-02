// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/blobstore"
)

// CacheService updates the space used cache.
//
// architecture: Chore
type CacheService struct {
	log                *zap.Logger
	usageCache         *BlobsUsageCache
	store              *Store
	pieceScanOnStartup bool
	Loop               *sync2.Cycle

	// InitFence is released once the cache's Run method returns or when it has
	// completed its first loop. This is useful for testing.
	InitFence sync2.Fence
}

// NewService creates a new cache service that updates the space usage cache on startup and syncs the cache values to
// persistent storage on an interval.
func NewService(log *zap.Logger, usageCache *BlobsUsageCache, pieces *Store, interval time.Duration, pieceScanOnStartup bool) *CacheService {
	return &CacheService{
		log:                log,
		usageCache:         usageCache,
		store:              pieces,
		pieceScanOnStartup: pieceScanOnStartup,
		Loop:               sync2.NewCycle(interval),
	}
}

// Run recalculates the space used cache once and also runs a loop to sync the space used cache
// to persistent storage on an interval.
func (service *CacheService) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer service.InitFence.Release()

	totalsAtStart := service.usageCache.copyCacheTotals()

	// recalculate the cache once
	if service.pieceScanOnStartup {
		piecesTotal, piecesContentSize, totalsBySatellite, err := service.store.SpaceUsedTotalAndBySatellite(ctx)
		if err != nil {
			service.log.Error("error getting current used space: ", zap.Error(err))
			return err
		}
		trashTotal, err := service.usageCache.Blobs.SpaceUsedForTrash(ctx)
		if err != nil {
			service.log.Error("error getting current used space for trash: ", zap.Error(err))
			return err
		}
		service.usageCache.Recalculate(
			piecesTotal,
			totalsAtStart.piecesTotal,
			piecesContentSize,
			totalsAtStart.piecesContentSize,
			trashTotal,
			totalsAtStart.trashTotal,
			totalsBySatellite,
			totalsAtStart.spaceUsedBySatellite,
		)
	} else {
		service.log.Info("Startup piece scan omitted by configuration")
	}

	if err = service.store.spaceUsedDB.Init(ctx); err != nil {
		service.log.Error("error during init space usage db: ", zap.Error(err))
		return err
	}

	return service.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		// on a loop sync the cache values to the db so that we have the them saved
		// in the case that the storagenode restarts
		if err := service.PersistCacheTotals(ctx); err != nil {
			service.log.Error("error persisting cache totals to the database: ", zap.Error(err))
		}
		service.InitFence.Release()
		return err
	})
}

// PersistCacheTotals saves the current totals of the space used cache to the database
// so that if the storagenode restarts it can retrieve the latest space used
// values without needing to recalculate since that could take a long time.
func (service *CacheService) PersistCacheTotals(ctx context.Context) error {
	cache := service.usageCache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if err := service.store.spaceUsedDB.UpdatePieceTotals(ctx, cache.piecesTotal, cache.piecesContentSize); err != nil {
		return err
	}
	if err := service.store.spaceUsedDB.UpdatePieceTotalsForAllSatellites(ctx, cache.spaceUsedBySatellite); err != nil {
		return err
	}
	if err := service.store.spaceUsedDB.UpdateTrashTotal(ctx, cache.trashTotal); err != nil {
		return err
	}
	return nil
}

// Init initializes the space used cache with the most recent values that were stored persistently.
func (service *CacheService) Init(ctx context.Context) (err error) {
	piecesTotal, piecesContentSize, err := service.store.spaceUsedDB.GetPieceTotals(ctx)
	if err != nil {
		service.log.Error("CacheServiceInit error during initializing space usage cache GetTotal:", zap.Error(err))
		return err
	}

	totalsBySatellite, err := service.store.spaceUsedDB.GetPieceTotalsForAllSatellites(ctx)
	if err != nil {
		service.log.Error("CacheServiceInit error during initializing space usage cache GetTotalsForAllSatellites:", zap.Error(err))
		return err
	}

	trashTotal, err := service.store.spaceUsedDB.GetTrashTotal(ctx)
	if err != nil {
		service.log.Error("CacheServiceInit error during initializing space usage cache GetTrashTotal:", zap.Error(err))
		return err
	}

	service.usageCache.init(piecesTotal, piecesContentSize, trashTotal, totalsBySatellite)
	return nil
}

// Close closes the loop.
func (service *CacheService) Close() (err error) {
	service.Loop.Close()
	return nil
}

// BlobsUsageCache is a blob storage with a cache for storing
// totals of current space used.
//
// The following names have the following meaning:
// - piecesTotal: the total space used by pieces, including headers
// - piecesContentSize: the space used by piece content, not including headers
// - trashTotal: the total space used in the trash, including headers
//
// pieceTotal and pieceContentSize are the corollary for a single file.
//
// architecture: Database
type BlobsUsageCache struct {
	blobstore.Blobs
	log *zap.Logger

	mu                   sync.Mutex
	piecesTotal          int64
	piecesContentSize    int64
	trashTotal           int64
	spaceUsedBySatellite map[storj.NodeID]SatelliteUsage
}

// NewBlobsUsageCache creates a new disk blob store with a space used cache.
func NewBlobsUsageCache(log *zap.Logger, blob blobstore.Blobs) *BlobsUsageCache {
	usageCache := &BlobsUsageCache{
		log:                  log,
		Blobs:                blob,
		spaceUsedBySatellite: map[storj.NodeID]SatelliteUsage{},
	}
	mon.Chain(usageCache)
	return usageCache
}

// NewBlobsUsageCacheTest creates a new disk blob store with a space used cache.
func NewBlobsUsageCacheTest(log *zap.Logger, blob blobstore.Blobs, piecesTotal, piecesContentSize, trashTotal int64, spaceUsedBySatellite map[storj.NodeID]SatelliteUsage) *BlobsUsageCache {
	return &BlobsUsageCache{
		log:                  log,
		Blobs:                blob,
		piecesTotal:          piecesTotal,
		piecesContentSize:    piecesContentSize,
		trashTotal:           trashTotal,
		spaceUsedBySatellite: spaceUsedBySatellite,
	}
}

func (blobs *BlobsUsageCache) init(pieceTotal, contentSize, trashTotal int64, totalsBySatellite map[storj.NodeID]SatelliteUsage) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	blobs.piecesTotal = pieceTotal
	blobs.piecesContentSize = contentSize
	blobs.trashTotal = trashTotal
	blobs.spaceUsedBySatellite = totalsBySatellite
}

// SpaceUsedBySatellite returns the current total space used for a specific
// satellite for all pieces.
func (blobs *BlobsUsageCache) SpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (piecesTotal int64, piecesContentSize int64, err error) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	values := blobs.spaceUsedBySatellite[satelliteID]
	return values.Total, values.ContentSize, nil
}

// SpaceUsedForPieces returns the current total used space for all pieces.
func (blobs *BlobsUsageCache) SpaceUsedForPieces(ctx context.Context) (int64, int64, error) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	return blobs.piecesTotal, blobs.piecesContentSize, nil
}

// SpaceUsedForTrash returns the current total used space for the trash dir.
func (blobs *BlobsUsageCache) SpaceUsedForTrash(ctx context.Context) (int64, error) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	return blobs.trashTotal, nil
}

// Delete gets the size of the piece that is going to be deleted then deletes it and
// updates the space used cache accordingly.
func (blobs *BlobsUsageCache) Delete(ctx context.Context, blobRef blobstore.BlobRef) error {
	pieceTotal, pieceContentSize, err := blobs.pieceSizes(ctx, blobRef)
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
	blobs.Update(ctx, satelliteID, -pieceTotal, -pieceContentSize, 0)
	blobs.log.Debug("deleted piece", zap.String("Satellite ID", satelliteID.String()), zap.Int64("disk space freed in bytes", pieceContentSize))
	return nil
}

// DeleteWithStorageFormat gets the size of the piece that is going to be deleted then deletes it and
// updates the space used cache accordingly.
func (blobs *BlobsUsageCache) DeleteWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion) error {
	pieceTotal, pieceContentSize, err := blobs.pieceSizes(ctx, ref)
	if err != nil {
		return Error.Wrap(err)
	}

	if err := blobs.Blobs.DeleteWithStorageFormat(ctx, ref, formatVer); err != nil {
		return Error.Wrap(err)
	}

	satelliteID, err := storj.NodeIDFromBytes(ref.Namespace)
	if err != nil {
		return err
	}

	blobs.Update(ctx, satelliteID, -pieceTotal, -pieceContentSize, 0)
	blobs.log.Debug("deleted piece", zap.String("Satellite ID", satelliteID.String()), zap.Any("Version", formatVer), zap.Int64("disk space freed in bytes", pieceContentSize))
	return nil
}

func (blobs *BlobsUsageCache) pieceSizes(ctx context.Context, blobRef blobstore.BlobRef) (pieceTotal int64, pieceContentSize int64, err error) {
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

// Update updates the cache totals.
func (blobs *BlobsUsageCache) Update(ctx context.Context, satelliteID storj.NodeID, piecesTotalDelta, piecesContentSizeDelta, trashDelta int64) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()

	blobs.piecesTotal += piecesTotalDelta
	blobs.piecesContentSize += piecesContentSizeDelta
	blobs.trashTotal += trashDelta

	blobs.ensurePositiveCacheValue(&blobs.piecesTotal, "piecesTotal")
	blobs.ensurePositiveCacheValue(&blobs.piecesContentSize, "piecesContentSize")
	blobs.ensurePositiveCacheValue(&blobs.trashTotal, "trashTotal")

	oldVals := blobs.spaceUsedBySatellite[satelliteID]
	newVals := SatelliteUsage{
		Total:       oldVals.Total + piecesTotalDelta,
		ContentSize: oldVals.ContentSize + piecesContentSizeDelta,
	}
	blobs.ensurePositiveCacheValue(&newVals.Total, "satPiecesTotal")
	blobs.ensurePositiveCacheValue(&newVals.ContentSize, "satPiecesContentSize")
	blobs.spaceUsedBySatellite[satelliteID] = newVals

}

func (blobs *BlobsUsageCache) ensurePositiveCacheValue(value *int64, name string) {
	if *value >= 0 {
		return
	}
	blobs.log.Error(fmt.Sprintf("%s < 0", name), zap.Int64(name, *value))
	*value = 0
}

// Trash moves the ref to the trash and updates the cache.
func (blobs *BlobsUsageCache) Trash(ctx context.Context, blobRef blobstore.BlobRef, timestamp time.Time) error {
	pieceTotal, pieceContentSize, err := blobs.pieceSizes(ctx, blobRef)
	if err != nil {
		return Error.Wrap(err)
	}

	err = blobs.Blobs.Trash(ctx, blobRef, timestamp)
	if err != nil {
		return Error.Wrap(err)
	}

	satelliteID, err := storj.NodeIDFromBytes(blobRef.Namespace)
	if err != nil {
		return Error.Wrap(err)
	}

	blobs.Update(ctx, satelliteID, -pieceTotal, -pieceContentSize, pieceTotal)
	return nil
}

// EmptyTrash empties the trash and updates the cache.
func (blobs *BlobsUsageCache) EmptyTrash(ctx context.Context, namespace []byte, trashedBefore time.Time) (int64, [][]byte, error) {
	satelliteID, err := storj.NodeIDFromBytes(namespace)
	if err != nil {
		return 0, nil, err
	}

	bytesEmptied, keys, err := blobs.Blobs.EmptyTrash(ctx, namespace, trashedBefore)
	if err != nil {
		return 0, nil, err
	}

	blobs.Update(ctx, satelliteID, 0, 0, -bytesEmptied)

	return bytesEmptied, keys, nil
}

// RestoreTrash restores the trash for the namespace and updates the cache.
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
		pieceTotal, pieceContentSize, sizeErr := blobs.pieceSizes(ctx, blobstore.BlobRef{
			Key:       key,
			Namespace: namespace,
		})
		if sizeErr != nil {
			err = errs.Combine(err, sizeErr)
			continue
		}
		blobs.Update(ctx, satelliteID, pieceTotal, pieceContentSize, -pieceTotal)
	}

	return keysRestored, err
}

// DeleteNamespace deletes all blobs for a satellite and updates the cache.
func (blobs *BlobsUsageCache) DeleteNamespace(ctx context.Context, namespace []byte) error {
	satelliteID, err := storj.NodeIDFromBytes(namespace)
	if err != nil {
		return err
	}

	piecesTotal, piecesContentSize, err := blobs.SpaceUsedBySatellite(ctx, satelliteID)
	if err != nil {
		return err
	}

	err = blobs.Blobs.DeleteNamespace(ctx, satelliteID.Bytes())
	if err != nil {
		return err
	}

	blobs.Update(ctx, satelliteID, -piecesTotal, -piecesContentSize, 0)
	return nil
}

func (blobs *BlobsUsageCache) copyCacheTotals() BlobsUsageCache {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	var copyMap = map[storj.NodeID]SatelliteUsage{}
	for k, v := range blobs.spaceUsedBySatellite {
		copyMap[k] = v
	}
	return BlobsUsageCache{
		piecesTotal:          blobs.piecesTotal,
		piecesContentSize:    blobs.piecesContentSize,
		trashTotal:           blobs.trashTotal,
		spaceUsedBySatellite: copyMap,
	}
}

// Recalculate estimates new totals for the space used cache. In order to get new totals for the
// space used cache, we had to iterate over all the pieces on disk. Since that can potentially take
// a long time, here we need to check if we missed any additions/deletions while we were iterating and
// estimate how many bytes missed then add those to the space used result of iteration.
func (blobs *BlobsUsageCache) Recalculate(
	piecesTotal,
	piecesTotalAtStart,
	piecesContentSize,
	piecesContentSizeAtStart,
	trashTotal,
	trashTotalAtStart int64,
	totalsBySatellite,
	totalsBySatelliteAtStart map[storj.NodeID]SatelliteUsage,
) {

	totalsAtEnd := blobs.copyCacheTotals()

	estimatedPiecesTotal := estimate(
		piecesTotal,
		piecesTotalAtStart,
		totalsAtEnd.piecesTotal,
	)

	estimatedTotalTrash := estimate(
		trashTotal,
		trashTotalAtStart,
		totalsAtEnd.trashTotal,
	)

	estimatedPiecesContentSize := estimate(
		piecesContentSize,
		piecesContentSizeAtStart,
		totalsAtEnd.piecesContentSize,
	)

	var estimatedTotalsBySatellite = map[storj.NodeID]SatelliteUsage{}
	for ID, values := range totalsBySatellite {
		estimatedTotal := estimate(
			values.Total,
			totalsBySatelliteAtStart[ID].Total,
			totalsAtEnd.spaceUsedBySatellite[ID].Total,
		)
		estimatedPiecesContentSize := estimate(
			values.ContentSize,
			totalsBySatelliteAtStart[ID].ContentSize,
			totalsAtEnd.spaceUsedBySatellite[ID].ContentSize,
		)
		// if the estimatedTotal is zero then there is no data stored
		// for this satelliteID so don't add it to the cache
		if estimatedTotal == 0 && estimatedPiecesContentSize == 0 {
			continue
		}
		estimatedTotalsBySatellite[ID] = SatelliteUsage{
			Total:       estimatedTotal,
			ContentSize: estimatedPiecesContentSize,
		}
	}

	// find any saIDs that are in totalsAtEnd but not in totalsBySatellite
	missedWhenIterationEnded := getMissed(totalsAtEnd.spaceUsedBySatellite,
		totalsBySatellite,
	)
	if len(missedWhenIterationEnded) > 0 {
		for ID := range missedWhenIterationEnded {
			estimatedTotal := estimate(
				0,
				totalsBySatelliteAtStart[ID].Total,
				totalsAtEnd.spaceUsedBySatellite[ID].Total,
			)
			estimatedPiecesContentSize := estimate(
				0,
				totalsBySatelliteAtStart[ID].ContentSize,
				totalsAtEnd.spaceUsedBySatellite[ID].ContentSize,
			)
			if estimatedTotal == 0 && estimatedPiecesContentSize == 0 {
				continue
			}
			estimatedTotalsBySatellite[ID] = SatelliteUsage{
				Total:       estimatedTotal,
				ContentSize: estimatedPiecesContentSize,
			}
		}
	}

	blobs.mu.Lock()
	blobs.piecesTotal = estimatedPiecesTotal
	blobs.piecesContentSize = estimatedPiecesContentSize
	blobs.trashTotal = estimatedTotalTrash
	blobs.spaceUsedBySatellite = estimatedTotalsBySatellite
	blobs.mu.Unlock()
}

func estimate(newSpaceUsedTotal, totalAtIterationStart, totalAtIterationEnd int64) int64 {
	if newSpaceUsedTotal == totalAtIterationEnd {
		if newSpaceUsedTotal < 0 {
			return 0
		}
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

func getMissed(endTotals, newTotals map[storj.NodeID]SatelliteUsage) map[storj.NodeID]SatelliteUsage {
	var missed = map[storj.NodeID]SatelliteUsage{}
	for id, vals := range endTotals {
		if _, ok := newTotals[id]; !ok {
			missed[id] = vals
		}
	}
	return missed
}

// Close satisfies the pieces interface.
func (blobs *BlobsUsageCache) Close() error {
	return nil
}

// TestCreateV0 creates a new V0 blob that can be written. This is only appropriate in test situations.
func (blobs *BlobsUsageCache) TestCreateV0(ctx context.Context, ref blobstore.BlobRef) (_ blobstore.BlobWriter, err error) {
	fStore := blobs.Blobs.(interface {
		TestCreateV0(ctx context.Context, ref blobstore.BlobRef) (_ blobstore.BlobWriter, err error)
	})
	return fStore.TestCreateV0(ctx, ref)
}

// Stats implements monkit.StatSource.
func (blobs *BlobsUsageCache) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	for satellite, used := range blobs.spaceUsedBySatellite {
		k := monkit.NewSeriesKey("blobs_usage").WithTag("satellite", satellite.String())
		cb(k, "total_size", float64(used.Total))
		cb(k, "content_size", float64(used.ContentSize))
	}
}
