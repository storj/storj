// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
)

// CacheData stores the amount of inline and allocated data
// for a bucket bandwidth rollup.
type CacheData struct {
	Inline    int64
	Allocated int64
	Settled   int64
}

// CacheKey is the key information for the cached map below.
type CacheKey struct {
	ProjectID  uuid.UUID
	BucketName string
	Action     pb.PieceAction
}

// RollupData contains the pending rollups waiting to be flushed to the db.
type RollupData map[CacheKey]CacheData

// RollupsWriteCache stores information needed to update bucket bandwidth rollups.
type RollupsWriteCache struct {
	DB
	batchSize int
	wg        sync.WaitGroup
	log       *zap.Logger

	mu             sync.Mutex
	pendingRollups RollupData
	currentSize    int
	latestTime     time.Time
	stopped        bool

	nextFlushCompletion *sync2.Fence
}

// NewRollupsWriteCache creates an RollupsWriteCache.
func NewRollupsWriteCache(log *zap.Logger, db DB, batchSize int) *RollupsWriteCache {
	return &RollupsWriteCache{
		DB:                  db,
		batchSize:           batchSize,
		log:                 log,
		pendingRollups:      make(RollupData),
		nextFlushCompletion: new(sync2.Fence),
	}
}

// UpdateBucketBandwidthAllocation updates the rollups cache adding allocated data for a bucket bandwidth rollup.
func (cache *RollupsWriteCache) UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error {
	return cache.updateCacheValue(ctx, projectID, bucketName, action, amount, 0, 0, intervalStart.UTC())
}

// UpdateBucketBandwidthInline updates the rollups cache adding inline data for a bucket bandwidth rollup.
func (cache *RollupsWriteCache) UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error {
	return cache.updateCacheValue(ctx, projectID, bucketName, action, 0, amount, 0, intervalStart.UTC())
}

// UpdateBucketBandwidthSettle updates the rollups cache adding settled data for a bucket bandwidth rollup.
func (cache *RollupsWriteCache) UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error {
	return cache.updateCacheValue(ctx, projectID, bucketName, action, 0, 0, amount, intervalStart.UTC())
}

// resetCache should only be called after you have acquired the cache lock. It
// will reset the various cache values and return the pendingRollups,
// latestTime, and currentSize.
func (cache *RollupsWriteCache) resetCache() (RollupData, time.Time, int) {
	pendingRollups := cache.pendingRollups
	cache.pendingRollups = make(RollupData)
	oldSize := cache.currentSize
	cache.currentSize = 0
	latestTime := cache.latestTime
	cache.latestTime = time.Time{}
	return pendingRollups, latestTime, oldSize
}

// Flush resets cache then flushes the everything in the rollups write cache to the database.
func (cache *RollupsWriteCache) Flush(ctx context.Context) {
	defer mon.Task()(&ctx)(nil)
	cache.mu.Lock()
	pendingRollups, latestTime, oldSize := cache.resetCache()
	cache.mu.Unlock()
	cache.flush(ctx, pendingRollups, latestTime, oldSize)
}

// CloseAndFlush flushes anything in the cache and marks the cache as stopped.
func (cache *RollupsWriteCache) CloseAndFlush(ctx context.Context) error {
	cache.mu.Lock()
	cache.stopped = true
	cache.mu.Unlock()
	cache.wg.Wait()

	cache.Flush(ctx)
	return nil
}

// flush flushes the everything in the rollups write cache to the database.
func (cache *RollupsWriteCache) flush(ctx context.Context, pendingRollups RollupData, latestTime time.Time, oldSize int) {
	defer mon.Task()(&ctx)(nil)

	rollups := make([]BucketBandwidthRollup, 0, oldSize)
	for cacheKey, cacheData := range pendingRollups {
		rollups = append(rollups, BucketBandwidthRollup{
			ProjectID:  cacheKey.ProjectID,
			BucketName: cacheKey.BucketName,
			Action:     cacheKey.Action,
			Inline:     cacheData.Inline,
			Allocated:  cacheData.Allocated,
			Settled:    cacheData.Settled,
		})
	}

	err := cache.DB.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		return tx.UpdateBucketBandwidthBatch(ctx, latestTime, rollups)
	})
	if err != nil {
		cache.log.Error("MONEY LOST! Bucket bandwidth rollup batch flush failed.", zap.Error(err))
	}

	var completion *sync2.Fence
	cache.mu.Lock()
	cache.nextFlushCompletion, completion = new(sync2.Fence), cache.nextFlushCompletion
	cache.mu.Unlock()
	completion.Release()
}

func (cache *RollupsWriteCache) updateCacheValue(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, allocated, inline, settled int64, intervalStart time.Time) error {
	defer mon.Task()(&ctx)(nil)

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.stopped {
		return Error.New("RollupsWriteCache is stopped")
	}

	if intervalStart.After(cache.latestTime) {
		cache.latestTime = intervalStart
	}

	key := CacheKey{
		ProjectID:  projectID,
		BucketName: string(bucketName),
		Action:     action,
	}

	data, ok := cache.pendingRollups[key]
	if !ok {
		cache.currentSize++
	}
	data.Allocated += allocated
	data.Inline += inline
	data.Settled += settled
	cache.pendingRollups[key] = data

	if cache.currentSize < cache.batchSize {
		return nil
	}
	pendingRollups, latestTime, oldSize := cache.resetCache()

	cache.wg.Add(1)
	go func() {
		cache.flush(ctx, pendingRollups, latestTime, oldSize)
		cache.wg.Done()
	}()

	return nil
}

// OnNextFlush waits until the next time a flush call is made, then closes
// the returned channel.
func (cache *RollupsWriteCache) OnNextFlush() <-chan struct{} {
	cache.mu.Lock()
	fence := cache.nextFlushCompletion
	cache.mu.Unlock()
	return fence.Done()
}

// CurrentSize returns the current size of the cache.
func (cache *RollupsWriteCache) CurrentSize() int {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.currentSize
}

// CurrentData returns the contents of the cache.
func (cache *RollupsWriteCache) CurrentData() RollupData {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	copyCache := RollupData{}
	for k, v := range cache.pendingRollups {
		copyCache[k] = v
	}
	return copyCache
}
