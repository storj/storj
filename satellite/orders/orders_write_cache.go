// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"bytes"
	"context"
	"sort"
	"sync"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/sync2"
)

// CacheData stores the amount of inline and allocated data
// for a bucket bandwidth rollup
type CacheData struct {
	Inline    int64
	Allocated int64
}

// CacheKey is the key information for the cached map below
type CacheKey struct {
	ProjectID  uuid.UUID
	BucketName string
	Action     pb.PieceAction
}

// RollupData contains the pending rollups waiting to be flushed to the db
type RollupData map[CacheKey]CacheData

// RollupsWriteCache stores information needed to update bucket bandwidth rollups
type RollupsWriteCache struct {
	DB
	batchSize   int
	currentSize int
	latestTime  time.Time

	log            *zap.Logger
	mu             sync.Mutex
	pendingRollups RollupData

	nextFlushCompletion *sync2.Fence
}

// NewRollupsWriteCache creates an RollupsWriteCache
func NewRollupsWriteCache(log *zap.Logger, db DB, batchSize int) *RollupsWriteCache {
	return &RollupsWriteCache{
		DB:                  db,
		batchSize:           batchSize,
		log:                 log,
		pendingRollups:      make(RollupData),
		nextFlushCompletion: new(sync2.Fence),
	}
}

// UpdateBucketBandwidthAllocation updates the rollups cache adding allocated data for a bucket bandwidth rollup
func (cache *RollupsWriteCache) UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error {
	cache.updateCacheValue(ctx, projectID, bucketName, action, amount, 0, intervalStart.UTC())
	return nil
}

// UpdateBucketBandwidthInline updates the rollups cache adding inline data for a bucket bandwidth rollup
func (cache *RollupsWriteCache) UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error {
	cache.updateCacheValue(ctx, projectID, bucketName, action, 0, amount, intervalStart.UTC())
	return nil
}

// FlushToDB resets cache then flushes the everything in the rollups write cache to the database
func (cache *RollupsWriteCache) FlushToDB(ctx context.Context) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	pendingRollups := cache.pendingRollups
	cache.pendingRollups = make(RollupData)
	oldSize := cache.currentSize
	cache.currentSize = 0
	latestTime := cache.latestTime
	cache.latestTime = time.Time{}
	go cache.flushToDB(ctx, pendingRollups, latestTime, oldSize)
}

// flushToDB flushes the everything in the rollups write cache to the database
func (cache *RollupsWriteCache) flushToDB(ctx context.Context, pendingRollups RollupData, latestTime time.Time, oldSize int) {
	rollups := make([]BandwidthRollup, 0, oldSize)
	for cacheKey, cacheData := range pendingRollups {
		rollups = append(rollups, BandwidthRollup{
			ProjectID:  cacheKey.ProjectID,
			BucketName: cacheKey.BucketName,
			Action:     cacheKey.Action,
			Inline:     cacheData.Inline,
			Allocated:  cacheData.Allocated,
		})
	}

	SortRollups(rollups)

	err := cache.DB.UpdateBucketBandwidthBatch(ctx, latestTime.UTC(), rollups)
	if err != nil {
		cache.log.Error("MONEY LOST! Bucket bandwidth rollup batch flush failed.", zap.Error(err))
	}

	var completion *sync2.Fence
	cache.mu.Lock()
	cache.nextFlushCompletion, completion = new(sync2.Fence), cache.nextFlushCompletion
	cache.mu.Unlock()
	completion.Release()
}

// SortRollups sorts the rollups
func SortRollups(rollups []BandwidthRollup) {
	sort.SliceStable(rollups, func(i, j int) bool {
		uuidCompare := bytes.Compare(rollups[i].ProjectID[:], rollups[j].ProjectID[:])
		switch {
		case uuidCompare == -1:
			return true
		case uuidCompare == 1:
			return false
		case rollups[i].BucketName < rollups[j].BucketName:
			return true
		case rollups[i].BucketName > rollups[j].BucketName:
			return false
		case rollups[i].Action < rollups[j].Action:
			return true
		case rollups[i].Action > rollups[j].Action:
			return false
		default:
			return false
		}
	})
}

func (cache *RollupsWriteCache) updateCacheValue(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, allocated, inline int64, intervalStart time.Time) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

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
	cache.pendingRollups[key] = data

	if cache.currentSize < cache.batchSize {
		return
	}
	pendingRollups := cache.pendingRollups
	cache.pendingRollups = make(RollupData)
	oldSize := cache.currentSize
	cache.currentSize = 0
	latestTime := cache.latestTime
	cache.latestTime = time.Time{}
	go cache.flushToDB(ctx, pendingRollups, latestTime, oldSize)
}

// OnNextFlush waits until the next time a flushToDB call is made, then closes
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
