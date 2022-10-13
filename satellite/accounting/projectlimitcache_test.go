// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"context"
	"testing"

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

		_, err = projectLimitCache.GetProjectBandwidthLimit(ctx, testProject.ID)
		require.NoError(t, err)
		// if the data isn't in the cache we call into the database to get it
		require.Equal(t, expectedCallCount, mdb.callCount)

		_, err = projectLimitCache.GetProjectBandwidthLimit(ctx, testProject.ID)
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
		accountingDB := saPeer.DB.ProjectAccounting()
		projectLimitCache := saPeer.ProjectLimits.Cache
		defaultUsageLimit := saPeer.Config.Console.UsageLimits.Storage.Free.Int64()
		defaultBandwidthLimit := saPeer.Config.Console.UsageLimits.Bandwidth.Free.Int64()
		defaultSegmentLimit := int64(1000000)
		dbDefaultLimits := accounting.ProjectLimits{Usage: &defaultUsageLimit, Bandwidth: &defaultBandwidthLimit, Segments: &defaultSegmentLimit}

		testProject, err := saPeer.DB.Console().Projects().Insert(ctx, &console.Project{Name: "test", OwnerID: testrand.UUID()})
		require.NoError(t, err)

		const (
			errorLimit             = 0
			expectedUsageLimit     = 1
			expectedBandwidthLimit = 2
			expectedSegmentLimit   = 3
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
			_, err = projectLimitCache.GetProjectLimits(ctx, projectID)
			assert.Error(t, err)

			actualStorageLimitFromSvc, err := projectUsageSvc.GetProjectStorageLimit(ctx, projectID)
			assert.Error(t, err)
			assert.EqualValues(t, errorLimit, actualStorageLimitFromSvc)

			// bandwidth
			actualBandwidthLimitFromCache, err := projectLimitCache.GetProjectBandwidthLimit(ctx, projectID)
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

			actualLimitsFromCache, err := projectLimitCache.GetProjectLimits(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.Equal(t, dbDefaultLimits, actualLimitsFromCache)

			actualStorageLimitFromDB, err := accountingDB.GetProjectStorageLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.Nil(t, actualStorageLimitFromDB)

			actualLimitFromCache, err := projectLimitCache.GetProjectLimits(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.EqualValues(t, *dbDefaultLimits.Usage, *actualLimitFromCache.Usage)

			actualStorageLimitFromSvc, err := projectUsageSvc.GetProjectStorageLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.EqualValues(t, *dbDefaultLimits.Usage, actualStorageLimitFromSvc)

			actualBandwidthLimitFromDB, err := accountingDB.GetProjectBandwidthLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			assert.Nil(t, actualBandwidthLimitFromDB)

			actualBandwidthLimitFromCache, err := projectLimitCache.GetProjectBandwidthLimit(ctx, testProject.ID)
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
			actualLimitFromCache, err := projectLimitCache.GetProjectLimits(ctx, testProject.ID)
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

			actualBandwidthLimitFromCache, err := projectLimitCache.GetProjectBandwidthLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			require.EqualValues(t, expectedBandwidthLimit, actualBandwidthLimitFromCache)

			actualBandwidthLimitFromSvc, err := projectUsageSvc.GetProjectBandwidthLimit(ctx, testProject.ID)
			assert.NoError(t, err)
			require.EqualValues(t, expectedBandwidthLimit, actualBandwidthLimitFromSvc)

			// segments
			actualSegmentLimitFromDB, err := accountingDB.GetProjectSegmentLimit(ctx, testProject.ID)
			require.NoError(t, err)
			require.EqualValues(t, expectedSegmentLimit, *actualSegmentLimitFromDB)
		})
	})
}
