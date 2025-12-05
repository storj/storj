// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live_test

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testredis"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
)

func TestAddGetProjectStorageAndBandwidthUsage(t *testing.T) {
	tests := []struct {
		backend string
	}{
		{
			backend: "redis",
		},
	}
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redis, err := testredis.Start(ctx)
	require.NoError(t, err)
	defer ctx.Check(redis.Close)

	for _, tt := range tests {
		tt := tt
		t.Run(tt.backend, func(t *testing.T) {
			ctx := testcontext.New(t)

			var config live.Config
			if tt.backend == "redis" {
				config = live.Config{
					StorageBackend: "redis://" + redis.Addr() + "?db=0",
				}
			}

			cache, err := live.OpenCache(ctx, zaptest.NewLogger(t).Named("live-accounting"), config)
			require.NoError(t, err)
			defer ctx.Check(cache.Close)

			populatedData, err := populateCache(ctx, cache)
			require.NoError(t, err)

			// make sure all of the "projects" got all space updates and got right totals
			for _, pdata := range populatedData {
				pdata := pdata

				t.Run("storage", func(t *testing.T) {
					spaceUsed, err := cache.GetProjectStorageUsage(ctx, pdata.projectID)
					require.NoError(t, err)
					assert.Equalf(t, pdata.storageSum, spaceUsed, "projectID %v", pdata.projectID)

					// upate it again and check
					negativeVal := -(rand.Int63n(pdata.storageSum) + 1)
					pdata.storageSum += negativeVal
					err = cache.UpdateProjectStorageAndSegmentUsage(ctx, pdata.projectID, negativeVal, 0)
					require.NoError(t, err)

					spaceUsed, err = cache.GetProjectStorageUsage(ctx, pdata.projectID)
					require.NoError(t, err)
					assert.EqualValues(t, pdata.storageSum, spaceUsed)
				})

				t.Run("bandwidth", func(t *testing.T) {
					bandwidthUsed, err := cache.GetProjectBandwidthUsage(ctx, pdata.projectID, pdata.bandwidthNow)
					require.NoError(t, err)
					assert.Equalf(t, pdata.bandwidthSum, bandwidthUsed, "projectID %v", pdata.projectID)

					// upate it again and check
					negativeVal := -(rand.Int63n(pdata.bandwidthSum) + 1)
					pdata.bandwidthSum += negativeVal
					err = cache.UpdateProjectBandwidthUsage(ctx, pdata.projectID, negativeVal, time.Second*2, pdata.bandwidthNow)
					require.NoError(t, err)

					bandwidthUsed, err = cache.GetProjectBandwidthUsage(ctx, pdata.projectID, pdata.bandwidthNow)
					require.NoError(t, err)
					assert.EqualValues(t, pdata.bandwidthSum, bandwidthUsed)
				})
			}
		})
	}
}

func TestGetAllProjectTotals(t *testing.T) {
	tests := []struct {
		backend string
	}{
		{
			backend: "redis",
		},
	}
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redis, err := testredis.Start(ctx)
	require.NoError(t, err)
	defer ctx.Check(redis.Close)

	for _, tt := range tests {
		tt := tt
		t.Run(tt.backend, func(t *testing.T) {
			ctx := testcontext.New(t)

			var config live.Config
			if tt.backend == "redis" {
				config = live.Config{
					StorageBackend: "redis://" + redis.Addr() + "?db=0",
				}
			}

			cache, err := live.OpenCache(ctx, zaptest.NewLogger(t).Named("live-accounting"), config)
			require.NoError(t, err)
			defer ctx.Check(cache.Close)

			projectIDs := make([]uuid.UUID, 1000)
			for i := range projectIDs {
				projectIDs[i] = testrand.UUID()
				increment := int64(i)
				err := cache.UpdateProjectStorageAndSegmentUsage(ctx, projectIDs[i], increment, increment)
				require.NoError(t, err)
			}

			for _, batchSize := range []int{1, 2, 3, 10, 13, 10000} {
				t.Run("batch-size-"+strconv.Itoa(batchSize), func(t *testing.T) {
					config.BatchSize = batchSize
					testCache, err := live.OpenCache(ctx, zaptest.NewLogger(t).Named("live-accounting"), config)
					require.NoError(t, err)
					defer ctx.Check(testCache.Close)

					usage, err := testCache.GetAllProjectTotals(ctx)
					require.NoError(t, err)
					require.Len(t, usage, len(projectIDs))

					// make sure each project ID and total was received
					for _, projID := range projectIDs {
						totalStorage, err := testCache.GetProjectStorageUsage(ctx, projID)
						require.NoError(t, err)
						assert.Equal(t, totalStorage, usage[projID].Storage)

						totalStorageBis, totalSegments, err := testCache.GetProjectStorageAndSegmentUsage(ctx, projID)
						require.NoError(t, err)
						assert.Equal(t, totalStorageBis, usage[projID].Storage)
						assert.Equal(t, totalSegments, usage[projID].Segments)
					}
				})
			}
		})
	}
}

func TestLiveAccountingCache_ProjectBandwidthUsage_expiration(t *testing.T) {
	tests := []struct {
		backend string
	}{
		{
			backend: "redis",
		},
	}
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redis, err := testredis.Start(ctx)
	require.NoError(t, err)

	defer ctx.Check(redis.Close)

	for _, tt := range tests {
		tt := tt
		t.Run(tt.backend, func(t *testing.T) {
			ctx := testcontext.New(t)

			var config live.Config
			if tt.backend == "redis" {
				config = live.Config{
					StorageBackend: "redis://" + redis.Addr() + "?db=0",
				}
			}

			cache, err := live.OpenCache(ctx, zaptest.NewLogger(t).Named("live-accounting"), config)
			require.NoError(t, err)
			defer ctx.Check(cache.Close)

			var (
				projectID = testrand.UUID()
				now       = time.Now()
			)
			err = cache.UpdateProjectBandwidthUsage(ctx, projectID, rand.Int63n(4096)+1, time.Second, now)
			require.NoError(t, err)

			if tt.backend == "redis" {
				redis.FastForward(time.Second)
			}

			time.Sleep(2 * time.Second)
			_, err = cache.GetProjectBandwidthUsage(ctx, projectID, now)
			require.Error(t, err)
		})
	}
}

func TestInsertProjectBandwidthUsage(t *testing.T) {
	tests := []struct {
		backend string
	}{
		{
			backend: "redis",
		},
	}
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redis, err := testredis.Start(ctx)
	require.NoError(t, err)

	defer ctx.Check(redis.Close)

	for _, tt := range tests {
		tt := tt
		t.Run(tt.backend, func(t *testing.T) {
			ctx := testcontext.New(t)

			var config live.Config
			if tt.backend == "redis" {
				config = live.Config{
					StorageBackend: "redis://" + redis.Addr() + "?db=0",
				}
			}

			cache, err := live.OpenCache(ctx, zaptest.NewLogger(t).Named("live-accounting"), config)
			require.NoError(t, err)
			defer ctx.Check(cache.Close)

			var (
				projectID = testrand.UUID()
				now       = time.Now()
			)
			expVal := rand.Int63n(4096) + 1
			inserted, err := cache.InsertProjectBandwidthUsage(ctx, projectID, expVal, time.Second, now)
			require.NoError(t, err)
			assert.True(t, inserted, "is inserted")

			val, err := cache.GetProjectBandwidthUsage(ctx, projectID, now)
			require.NoError(t, err)
			require.Equal(t, expVal, val, "inserted value")

			{ // This shouldn't set the value because it already exists.
				newVal := rand.Int63n(4096) + 1
				inserted, err := cache.InsertProjectBandwidthUsage(ctx, projectID, newVal, time.Second, now)
				require.NoError(t, err)
				assert.False(t, inserted, "is inserted")

				val, err := cache.GetProjectBandwidthUsage(ctx, projectID, now)
				require.NoError(t, err)
				require.Equal(t, expVal, val, "inserted value")
			}

			if tt.backend == "redis" {
				redis.FastForward(time.Second)
			}

			time.Sleep(2 * time.Second)
			_, err = cache.GetProjectBandwidthUsage(ctx, projectID, now)
			require.Error(t, err)
		})
	}
}

type populateCacheData struct {
	projectID    uuid.UUID
	storageSum   int64
	bandwidthSum int64
	bandwidthNow time.Time
}

func populateCache(ctx context.Context, cache accounting.Cache) ([]populateCacheData, error) {
	var (
		valuesListSize           = rand.Intn(10) + 10
		numProjects              = rand.Intn(100) + 100
		valueStorageMultiplier   = rand.Int63n(4095) + 1
		valueBandwdithMultiplier = rand.Int63n(4095) + 1
	)
	// make a largish list of varying values
	baseValues := make([]int64, valuesListSize)
	for i := range baseValues {
		baseValues[i] = rand.Int63n(int64(valuesListSize)) + 1
	}

	// make up some project IDs
	populatedData := make([]populateCacheData, numProjects)
	for i := range populatedData {
		populatedData[i] = populateCacheData{
			projectID: testrand.UUID(),
		}
	}

	// send lots of space used updates for all of these projects to the live
	// accounting store.
	errg, ctx := errgroup.WithContext(ctx)
	for i, pdata := range populatedData {
		var (
			i      = i
			projID = pdata.projectID
		)

		errg.Go(func() error {
			// have each project sending the values in a different order
			myValues := make([]int64, valuesListSize)
			copy(myValues, baseValues)
			rand.Shuffle(valuesListSize, func(v1, v2 int) {
				myValues[v1], myValues[v2] = myValues[v2], myValues[v1]
			})

			now := time.Now()
			populatedData[i].bandwidthNow = now

			for _, val := range myValues {
				storageVal := val * valueStorageMultiplier
				populatedData[i].storageSum += storageVal
				if err := cache.UpdateProjectStorageAndSegmentUsage(ctx, projID, storageVal, 0); err != nil {
					return err
				}

				bandwidthVal := val * valueBandwdithMultiplier
				populatedData[i].bandwidthSum += bandwidthVal
				if err := cache.UpdateProjectBandwidthUsage(ctx, projID, bandwidthVal, time.Hour, now); err != nil {
					return err
				}

			}
			return nil
		})
	}

	return populatedData, errg.Wait()
}
