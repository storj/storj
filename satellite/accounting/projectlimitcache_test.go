// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

type mockDB struct {
	callCount int
}

func (mdb *mockDB) GetProjectLimits(ctx context.Context, projectID uuid.UUID) (accounting.ProjectLimits, error) {
	mdb.callCount++
	return accounting.ProjectLimits{}, nil
}
func TestProjectLimitCacheCallCount(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		mdb := mockDB{}
		projectLimitCache := accounting.NewProjectLimitCache(&mdb, 0, 0, 0, accounting.ProjectLimitConfig{CacheCapacity: 100})

		testProject, err := db.Console().Projects().Insert(ctx, &console.Project{Name: "test", OwnerID: testrand.UUID()})
		require.NoError(t, err)

		const expectedCallCount = 1

		_, err = projectLimitCache.GetBandwidthLimit(ctx, testProject.ID)
		require.NoError(t, err)
		// if the data isn't in the cache we call into the database to get it
		require.Equal(t, expectedCallCount, mdb.callCount)

		_, err = projectLimitCache.GetBandwidthLimit(ctx, testProject.ID)
		require.NoError(t, err)
		// call count should still be 1 since the data is in the cache and we don't need
		// to get it from the db
		require.Equal(t, expectedCallCount, mdb.callCount)
	})
}

func TestProjectLimitCache(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		saPeer := planet.Satellites[0]
		projectUsageSvc := saPeer.Accounting.ProjectUsage
		projects := saPeer.DB.Console().Projects()
		accountingDB := saPeer.DB.ProjectAccounting()
		projectLimitCache := saPeer.ProjectLimits.Cache
		defaultUsageLimit := saPeer.Config.Console.UsageLimits.Storage.Free.Int64()
		defaultBandwidthLimit := saPeer.Config.Console.UsageLimits.Bandwidth.Free.Int64()
		defaultSegmentLimit := int64(1000000)
		dbDefaultLimits := accounting.ProjectLimits{
			Usage:      &defaultUsageLimit,
			Bandwidth:  &defaultBandwidthLimit,
			Segments:   &defaultSegmentLimit,
			RateLimit:  nil,
			BurstLimit: nil,
		}

		testProject, err := saPeer.DB.Console().Projects().Insert(ctx, &console.Project{Name: "test", OwnerID: testrand.UUID()})
		require.NoError(t, err)

		secondTestProject, err := saPeer.DB.Console().Projects().Insert(ctx, &console.Project{Name: "second project", OwnerID: testrand.UUID()})
		require.NoError(t, err)

		const (
			errorLimit             = 0
			expectedUsageLimit     = 1
			expectedBandwidthLimit = 2
			expectedSegmentLimit   = 3
			expectedRateLimit      = 4
			expectedBurstLimit     = 5
		)

		t.Run("project ID doesn't exist", func(t *testing.T) {
			projectID := testrand.UUID()
			actualStorageLimitFromDB, err := accountingDB.GetProjectStorageLimit(ctx, projectID)
			assert.Error(t, err)
			assert.Nil(t, actualStorageLimitFromDB)

			actualLimitsFromDB, err := accountingDB.GetProjectLimits(ctx, projectID)
			assert.Error(t, err)
			assert.Equal(t, accounting.ProjectLimits{}, actualLimitsFromDB)

			// storage
			_, err = projectLimitCache.GetLimits(ctx, projectID)
			assert.Error(t, err)

			actualStorageLimitFromSvc, err := projectUsageSvc.GetProjectStorageLimit(ctx, projectID)
			assert.Error(t, err)
			assert.EqualValues(t, errorLimit, actualStorageLimitFromSvc)

			// bandwidth
			actualBandwidthLimitFromCache, err := projectLimitCache.GetBandwidthLimit(ctx, projectID)
			assert.Error(t, err)
			assert.EqualValues(t, errorLimit, actualBandwidthLimitFromCache)

			actualBandwidthLimitFromSvc, err := projectUsageSvc.GetProjectBandwidthLimit(ctx, projectID)
			assert.Error(t, err)
			assert.EqualValues(t, errorLimit, actualBandwidthLimitFromSvc)
		})

		t.Run("default limits", func(t *testing.T) {
			actualLimitsFromDB, err := accountingDB.GetProjectLimits(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.Equal(t, accounting.ProjectLimits{
				Segments: &defaultSegmentLimit,
			}, actualLimitsFromDB)

			actualLimitsFromCache, err := projectLimitCache.GetLimits(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.Equal(t, dbDefaultLimits, actualLimitsFromCache)

			actualStorageLimitFromDB, err := accountingDB.GetProjectStorageLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.Nil(t, actualStorageLimitFromDB)

			actualLimitFromCache, err := projectLimitCache.GetLimits(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.EqualValues(t, *dbDefaultLimits.Usage, *actualLimitFromCache.Usage)

			actualStorageLimitFromSvc, err := projectUsageSvc.GetProjectStorageLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.EqualValues(t, *dbDefaultLimits.Usage, actualStorageLimitFromSvc)

			actualBandwidthLimitFromDB, err := accountingDB.GetProjectBandwidthLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.Nil(t, actualBandwidthLimitFromDB)

			actualBandwidthLimitFromCache, err := projectLimitCache.GetBandwidthLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.EqualValues(t, *dbDefaultLimits.Bandwidth, actualBandwidthLimitFromCache)

			actualBandwidthLimitFromSvc, err := projectUsageSvc.GetProjectBandwidthLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.EqualValues(t, *dbDefaultLimits.Bandwidth, actualBandwidthLimitFromSvc)
		})

		t.Run("update limits in the database", func(t *testing.T) {
			err = accountingDB.UpdateProjectUsageLimit(ctx, testProject.ID, expectedUsageLimit)
			require.NoError(t, err)
			err = accountingDB.UpdateProjectBandwidthLimit(ctx, testProject.ID, expectedBandwidthLimit)
			require.NoError(t, err)
			err = accountingDB.UpdateProjectSegmentLimit(ctx, testProject.ID, expectedSegmentLimit)
			require.NoError(t, err)

			actualStorageLimitFromDB, err := accountingDB.GetProjectStorageLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			require.EqualValues(t, expectedUsageLimit, *actualStorageLimitFromDB)

			actualLimitsFromDB, err := accountingDB.GetProjectLimits(ctx, testProject.ID)
			assert.NoError(t, err)
			usageLimits := int64(expectedUsageLimit)
			bwLimits := int64(expectedBandwidthLimit)
			segmentsLimits := int64(expectedSegmentLimit)
			assert.Equal(t, accounting.ProjectLimits{Usage: &usageLimits, Bandwidth: &bwLimits, Segments: &segmentsLimits}, actualLimitsFromDB)

			// storage
			actualLimitFromCache, err := projectLimitCache.GetLimits(ctx, testProject.ID)
			assert.NoError(t, err)
			require.EqualValues(t, expectedUsageLimit, *actualLimitFromCache.Usage)
			require.EqualValues(t, expectedSegmentLimit, *actualLimitFromCache.Segments)

			actualStorageLimitFromSvc, err := projectUsageSvc.GetProjectStorageLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			require.EqualValues(t, expectedUsageLimit, actualStorageLimitFromSvc)

			// bandwidth
			actualBandwidthLimitFromDB, err := accountingDB.GetProjectBandwidthLimit(ctx, testProject.ID)
			require.NoError(t, err)
			require.EqualValues(t, expectedBandwidthLimit, *actualBandwidthLimitFromDB)

			actualBandwidthLimitFromCache, err := projectLimitCache.GetBandwidthLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			require.EqualValues(t, expectedBandwidthLimit, actualBandwidthLimitFromCache)

			actualBandwidthLimitFromSvc, err := projectUsageSvc.GetProjectBandwidthLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			require.EqualValues(t, expectedBandwidthLimit, actualBandwidthLimitFromSvc)

			// segments
			actualSegmentLimitFromDB, err := accountingDB.GetProjectSegmentLimit(ctx, testProject.ID)
			require.NoError(t, err)
			require.EqualValues(t, expectedSegmentLimit, *actualSegmentLimitFromDB)

			// rate and burst limit
			require.NoError(t, projects.UpdateRateLimit(ctx, secondTestProject.ID, expectedRateLimit))
			require.NoError(t, projects.UpdateBurstLimit(ctx, secondTestProject.ID, expectedBurstLimit))

			limits, err := projectLimitCache.GetLimits(ctx, secondTestProject.ID)
			require.NoError(t, err)
			require.EqualValues(t, expectedRateLimit, *limits.RateLimit)
			require.EqualValues(t, expectedBurstLimit, *limits.BurstLimit)
		})

		t.Run("cache is used", func(t *testing.T) {
			require.NoError(t, accountingDB.UpdateProjectUsageLimit(ctx, testProject.ID, 1))
			require.NoError(t, accountingDB.UpdateProjectBandwidthLimit(ctx, testProject.ID, 2))
			require.NoError(t, accountingDB.UpdateProjectSegmentLimit(ctx, testProject.ID, 3))

			projectLimitCache := accounting.NewProjectLimitCache(accountingDB, 0, 0, 0, accounting.ProjectLimitConfig{
				CacheCapacity:   10,
				CacheExpiration: 60 * time.Second,
			})

			// fill cache with values from DB
			beforeCachedLimits, err := projectLimitCache.GetLimits(ctx, testProject.ID)
			require.NoError(t, err)

			// update limits in DB but not in cache
			require.NoError(t, accountingDB.UpdateProjectUsageLimit(ctx, testProject.ID, 4))
			require.NoError(t, accountingDB.UpdateProjectBandwidthLimit(ctx, testProject.ID, 5))
			require.NoError(t, accountingDB.UpdateProjectSegmentLimit(ctx, testProject.ID, 6))

			// verify that old values are still cached because expiration time was not reached yet
			afterCachedLimits, err := projectLimitCache.GetLimits(ctx, testProject.ID)
			require.NoError(t, err)

			require.Equal(t, beforeCachedLimits, afterCachedLimits)
		})
	})
}
