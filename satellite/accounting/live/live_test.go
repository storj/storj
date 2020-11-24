// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/storage/redis/redisserver"
)

func TestLiveAccountingCache(t *testing.T) {
	tests := []struct {
		backend string
	}{
		{
			backend: "redis",
		},
	}
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redis, err := redisserver.Mini(ctx)
	require.NoError(t, err)
	defer ctx.Check(redis.Close)

	for _, tt := range tests {
		var config live.Config
		if tt.backend == "redis" {
			config = live.Config{
				StorageBackend: "redis://" + redis.Addr() + "?db=0",
			}
		}

		cache, err := live.NewCache(zaptest.NewLogger(t).Named("live-accounting"), config)
		require.NoError(t, err)

		projectIDs, sum, err := populateCache(ctx, cache)
		require.NoError(t, err)

		// make sure all of the "projects" got all space updates and got right totals
		for _, projID := range projectIDs {
			spaceUsed, err := cache.GetProjectStorageUsage(ctx, projID)
			require.NoError(t, err)
			assert.Equalf(t, sum, spaceUsed, "projectID %v", projID)
		}

		negativeVal := int64(-100)
		sum += negativeVal

		for _, projID := range projectIDs {
			err = cache.AddProjectStorageUsage(ctx, projID, negativeVal)
			require.NoError(t, err)

			spaceUsed, err := cache.GetProjectStorageUsage(ctx, projID)
			require.NoError(t, err)
			assert.EqualValues(t, sum, spaceUsed)
		}
	}
}

func TestRedisCacheConcurrency(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redis, err := redisserver.Mini(ctx)
	require.NoError(t, err)
	defer ctx.Check(redis.Close)

	config := live.Config{
		StorageBackend: "redis://" + redis.Addr() + "?db=0",
	}
	cache, err := live.NewCache(zaptest.NewLogger(t).Named("live-accounting"), config)
	require.NoError(t, err)

	projectID := testrand.UUID()

	const (
		numConcurrent = 100
		spaceUsed     = 10
	)
	expectedSum := spaceUsed * numConcurrent

	var group errgroup.Group
	for i := 0; i < numConcurrent; i++ {
		group.Go(func() error {
			return cache.AddProjectStorageUsage(ctx, projectID, spaceUsed)
		})
	}
	require.NoError(t, group.Wait())

	total, err := cache.GetProjectStorageUsage(ctx, projectID)
	require.NoError(t, err)

	require.EqualValues(t, expectedSum, total)
}

func populateCache(ctx context.Context, cache accounting.Cache) (projectIDs []uuid.UUID, sum int64, _ error) {
	const (
		valuesListSize  = 10
		valueMultiplier = 4096
		numProjects     = 100
	)
	// make a largish list of varying values
	someValues := make([]int64, valuesListSize)
	for i := range someValues {
		someValues[i] = int64((i + 1) * valueMultiplier)
		sum += someValues[i]
	}

	// make up some project IDs
	projectIDs = make([]uuid.UUID, numProjects)
	for i := range projectIDs {
		projectIDs[i] = testrand.UUID()
	}

	// send lots of space used updates for all of these projects to the live
	// accounting store.
	errg, ctx := errgroup.WithContext(context.Background())
	for _, projID := range projectIDs {
		projID := projID
		errg.Go(func() error {
			// have each project sending the values in a different order
			myValues := make([]int64, valuesListSize)
			copy(myValues, someValues)
			rand.Shuffle(valuesListSize, func(v1, v2 int) {
				myValues[v1], myValues[v2] = myValues[v2], myValues[v1]
			})

			for _, val := range myValues {
				if err := cache.AddProjectStorageUsage(ctx, projID, val); err != nil {
					return err
				}
			}
			return nil
		})
	}

	return projectIDs, sum, errg.Wait()
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

	redis, err := redisserver.Mini(ctx)
	require.NoError(t, err)
	defer ctx.Check(redis.Close)

	for _, tt := range tests {
		var config live.Config
		if tt.backend == "redis" {
			config = live.Config{
				StorageBackend: "redis://" + redis.Addr() + "?db=0",
			}
		}

		cache, err := live.NewCache(zaptest.NewLogger(t).Named("live-accounting"), config)
		require.NoError(t, err)

		projectIDs := make([]uuid.UUID, 1000)
		for i := range projectIDs {
			projectIDs[i] = testrand.UUID()
			err := cache.AddProjectStorageUsage(ctx, projectIDs[i], int64(i))
			require.NoError(t, err)
		}

		projectTotals, err := cache.GetAllProjectTotals(ctx)
		require.NoError(t, err)
		require.Len(t, projectTotals, len(projectIDs))

		// make sure each project ID and total was received
		for _, projID := range projectIDs {
			total, err := cache.GetProjectStorageUsage(ctx, projID)
			require.NoError(t, err)
			assert.Equal(t, total, projectTotals[projID])
		}
	}
}
