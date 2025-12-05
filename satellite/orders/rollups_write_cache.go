// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/context2"
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
	Dead      int64
}

// CacheKey is the key information for the cached map below.
type CacheKey struct {
	ProjectID     uuid.UUID
	BucketName    string
	Action        pb.PieceAction
	IntervalStart int64
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
	stopped        bool
	flushing       bool

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
	return cache.updateCacheValue(ctx, projectID, bucketName, action, amount, 0, 0, 0, intervalStart.UTC())
}

// UpdateBucketBandwidthInline updates the rollups cache adding inline data for a bucket bandwidth rollup.
func (cache *RollupsWriteCache) UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error {
	return cache.updateCacheValue(ctx, projectID, bucketName, action, 0, amount, 0, 0, intervalStart.UTC())
}

// UpdateBucketBandwidthSettle updates the rollups cache adding settled data for a bucket bandwidth rollup - deadAmount is not used.
func (cache *RollupsWriteCache) UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, settledAmount, deadAmount int64, intervalStart time.Time) error {
	return cache.updateCacheValue(ctx, projectID, bucketName, action, 0, 0, settledAmount, deadAmount, intervalStart.UTC())
}

// resetCache should only be called after you have acquired the cache lock. It
// will reset the various cache values and return the pendingRollups.
func (cache *RollupsWriteCache) resetCache() RollupData {
	pendingRollups := cache.pendingRollups
	cache.pendingRollups = make(RollupData)

	return pendingRollups
}

// Flush resets cache then flushes the everything in the rollups write cache to the database.
func (cache *RollupsWriteCache) Flush(ctx context.Context) {
	defer mon.Task()(&ctx)(nil)

	cache.mu.Lock()

	// while we're already flushing, wait for it to complete.
	for cache.flushing {
		done := cache.nextFlushCompletion.Done()
		cache.mu.Unlock()

		select {
		case <-done:
		case <-ctx.Done():
			return
		}

		cache.mu.Lock()
	}

	cache.flushing = true
	pendingRollups := cache.resetCache()

	cache.mu.Unlock()

	cache.flush(ctx, pendingRollups)
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
func (cache *RollupsWriteCache) flush(ctx context.Context, pendingRollups RollupData) {
	defer mon.Task()(&ctx)(nil)

	if len(pendingRollups) > 0 {
		rollups := make([]BucketBandwidthRollup, 0, len(pendingRollups))
		for cacheKey, cacheData := range pendingRollups {
			rollups = append(rollups, BucketBandwidthRollup{
				ProjectID:     cacheKey.ProjectID,
				BucketName:    cacheKey.BucketName,
				IntervalStart: time.Unix(cacheKey.IntervalStart, 0),
				Action:        cacheKey.Action,
				Inline:        cacheData.Inline,
				Allocated:     cacheData.Allocated,
				Settled:       cacheData.Settled,
				Dead:          cacheData.Dead,
			})
		}

		// we would like to update bandwidth even if context was canceled. flushing
		// is triggered by endpoint methods (metainfo/orders) but flushing is started
		// in separate goroutine and because of that endpoint request can be finished
		// and its context will be canceled before UpdateBandwidthBatch is finished.
		ctx = context2.WithoutCancellation(ctx)

		err := cache.DB.UpdateBandwidthBatch(ctx, rollups)
		if err != nil {
			mon.Event("rollups_write_cache_flush_lost")

			// With error log only GET bandwidth because it's what we care most as we charge users for this.
			var settled int64
			var inline int64
			for _, rollup := range rollups {
				if rollup.Action == pb.PieceAction_GET {
					settled += rollup.Settled
					inline += rollup.Inline
				}
			}

			cache.log.Error("MONEY LOST! Bucket bandwidth rollup batch flush failed", zap.Int64("settled", settled), zap.Int64("inline", inline), zap.Error(err))
		}
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.nextFlushCompletion.Release()
	cache.nextFlushCompletion = new(sync2.Fence)
	cache.flushing = false
}

func (cache *RollupsWriteCache) updateCacheValue(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, allocated, inline, settled, dead int64, intervalStart time.Time) error {
	defer mon.Task()(&ctx)(nil)

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.stopped {
		return Error.New("RollupsWriteCache is stopped")
	}

	key := CacheKey{
		ProjectID:     projectID,
		BucketName:    string(bucketName),
		Action:        action,
		IntervalStart: time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), intervalStart.Hour(), 0, 0, 0, intervalStart.Location()).Unix(),
	}

	// prevent unbounded memory growth if we're not flushing fast enough
	// to keep up with incoming writes.
	data, ok := cache.pendingRollups[key]
	if !ok && len(cache.pendingRollups) >= cache.batchSize {
		mon.Event("rollups_write_cache_update_lost")
		cache.log.Error("MONEY LOST! Flushing too slow to keep up with demand",
			zap.Stringer("ProjectID", projectID),
			zap.Stringer("Action", action),
			zap.Int64("Allocated", allocated),
			zap.Int64("Inline", inline),
			zap.Int64("Settled", settled),
		)
	} else {

		data.Allocated += allocated
		data.Inline += inline
		data.Settled += settled
		data.Dead += dead
		cache.pendingRollups[key] = data
	}

	if len(cache.pendingRollups) < cache.batchSize {
		return nil
	}

	if !cache.flushing {
		cache.flushing = true
		pendingRollups := cache.resetCache()

		cache.wg.Add(1)
		go func() {
			defer cache.wg.Done()
			cache.flush(ctx, pendingRollups)
		}()
	}

	return nil
}

// OnNextFlush waits until the next time a flush call is made, then closes
// the returned channel.
func (cache *RollupsWriteCache) OnNextFlush() <-chan struct{} {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	return cache.nextFlushCompletion.Done()
}

// CurrentSize returns the current size of the cache.
func (cache *RollupsWriteCache) CurrentSize() int {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	return len(cache.pendingRollups)
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
