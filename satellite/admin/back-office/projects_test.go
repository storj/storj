// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	admin "storj.io/storj/satellite/admin/back-office"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestGetProject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.LiveAccounting.AsOfSystemInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		t.Run("unexisting project", func(t *testing.T) {
			sat := planet.Satellites[0]

			service := sat.Admin.Admin.Service
			_, apiErr := service.GetProject(ctx, testrand.UUID())
			require.Error(t, apiErr.Err)
			assert.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("existing user", func(t *testing.T) {
			consoleUser := &console.User{
				ID:               testrand.UUID(),
				FullName:         "Test User",
				Email:            "test@storj.io",
				PasswordHash:     testrand.Bytes(8),
				Status:           console.Inactive,
				UserAgent:        []byte("agent"),
				DefaultPlacement: 5,
			}

			sat := planet.Satellites[0]
			consoleDB := sat.DB.Console()
			_, err := consoleDB.Users().Insert(ctx, consoleUser)
			require.NoError(t, err)

			consoleUser.Status = console.Active
			require.NoError(
				t,
				consoleDB.Users().Update(ctx, consoleUser.ID, console.UpdateUserRequest{Status: &consoleUser.Status}),
			)

			// Project with default limits
			// Note: the projects DB Insert method will set storage, segment, bandwidth limits to whatever is passed in.
			// If nothing is passed, they are nil. In production, a project is created by the console service, which calls
			// this DB method with the default limits passed.
			projID := testrand.UUID()
			consoleProject := &console.Project{
				ID:             projID,
				Name:           "project-free-account",
				Description:    "This is a project created at the time that owner's user account is a free account",
				OwnerID:        consoleUser.ID,
				StorageLimit:   &sat.Config.Console.UsageLimits.Storage.Free,
				BandwidthLimit: &sat.Config.Console.UsageLimits.Bandwidth.Free,
				SegmentLimit:   &sat.Config.Console.UsageLimits.Segment.Free,
			}

			consoleProject, err = consoleDB.Projects().Insert(ctx, consoleProject)
			require.NoError(t, err)
			projPublicID := consoleProject.PublicID

			service := sat.Admin.Admin.Service
			for _, id := range []uuid.UUID{consoleProject.ID, consoleProject.PublicID} {
				project, apiErr := service.GetProject(ctx, id)
				require.NoError(t, apiErr.Err)
				assert.Equal(t, consoleProject.ID, project.ID)
				assert.Equal(t, consoleProject.PublicID, project.PublicID)
				assert.Equal(t, consoleProject.Name, project.Name)
				assert.Equal(t, consoleProject.Description, project.Description)
				assert.EqualValues(t, consoleProject.UserAgent, project.UserAgent)
				assert.Equal(t, consoleProject.CreatedAt, project.CreatedAt)
			}

			project, apiErr := service.GetProject(ctx, projPublicID)
			require.NoError(t, apiErr.Err)

			// service should return defaults for these since they are null in DB.
			defaultRate := int(sat.Config.Metainfo.RateLimiter.Rate)
			defaultBurst := defaultRate
			defaultMaxBuckets := sat.Config.Metainfo.ProjectLimits.MaxBuckets

			// check DB value is null and admin value is not null.
			require.Nil(t, consoleProject.RateLimit)
			require.NotNil(t, project.RateLimit)
			assert.Equal(t, defaultRate, *project.RateLimit)
			require.Nil(t, consoleProject.BurstLimit)
			require.NotNil(t, project.BurstLimit)
			assert.Equal(t, defaultBurst, *project.BurstLimit)
			require.Nil(t, consoleProject.MaxBuckets)
			require.NotNil(t, project.MaxBuckets)
			assert.Equal(t, defaultMaxBuckets, *project.MaxBuckets)

			assert.Equal(t, consoleProject.OwnerID, project.Owner.ID)
			assert.Equal(t, consoleUser.FullName, project.Owner.FullName)
			assert.Equal(t, consoleUser.Email, project.Owner.Email)

			assert.EqualValues(t, consoleProject.BandwidthLimit, project.BandwidthLimit)
			assert.Zero(t, project.BandwidthUsed)
			assert.EqualValues(t, consoleProject.StorageLimit, project.StorageLimit)
			require.NotNil(t, project.StorageUsed)
			assert.Zero(t, *project.StorageUsed)
			assert.EqualValues(t, consoleProject.SegmentLimit, project.SegmentLimit)
			require.NotNil(t, project.SegmentUsed)
			assert.Zero(t, *project.SegmentUsed)

			// now set the null columns to specific values and check admin returns them.
			newBucketLimit := defaultMaxBuckets * 2
			require.NoError(t, consoleDB.Projects().UpdateBucketLimit(ctx, projID, &newBucketLimit))

			newRateLimit := defaultRate * 2
			require.NoError(t, consoleDB.Projects().UpdateRateLimit(ctx, projID, &newRateLimit))

			// set burst to different value than rate to make sure they are returned correctly (burst == rate by default)
			newBurstLimit := defaultBurst * 3
			require.NoError(t, consoleDB.Projects().UpdateBurstLimit(ctx, projID, &newBurstLimit))

			project, apiErr = service.GetProject(ctx, projPublicID)
			require.NoError(t, apiErr.Err)

			require.NotNil(t, project.MaxBuckets)
			require.Equal(t, newBucketLimit, *project.MaxBuckets)
			require.NotNil(t, project.RateLimit)
			require.Equal(t, newRateLimit, *project.RateLimit)
			require.NotNil(t, project.BurstLimit)
			require.Equal(t, newBurstLimit, *project.BurstLimit)

			// Create usage
			bucket, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      testrand.BucketName(),
				ProjectID: projID,
			})
			require.NoError(t, err)

			obj := metabasetest.CreateObject(ctx, t, sat.Metabase.DB, metabase.ObjectStream{
				ProjectID:  projID,
				BucketName: metabase.BucketName(bucket.Name),
				ObjectKey:  metabasetest.RandObjectKey(),
				Version:    12345,
				StreamID:   testrand.UUID(),
			}, 16)

			usedStorage := obj.TotalEncryptedSize
			usedSegments := int64(obj.SegmentCount)

			usedBandwidth := int64(2000)
			err = sat.DB.Orders().
				UpdateBucketBandwidthAllocation(ctx, projID, []byte(bucket.Name), pb.PieceAction_GET, usedBandwidth, time.Now())
			require.NoError(t, err)
			sat.Accounting.Tally.Loop.TriggerWait()

			project, apiErr = service.GetProject(ctx, projPublicID)
			require.NoError(t, apiErr.Err)

			assert.Equal(t, usedBandwidth, project.BandwidthUsed)
			require.NotNil(t, project.StorageUsed)
			assert.Equal(t, usedStorage, *project.StorageUsed)
			require.NotNil(t, project.SegmentUsed)
			assert.Equal(t, usedSegments, *project.SegmentUsed)
		})
	})
}

func TestUpdateProjectLimits(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.LiveAccounting.AsOfSystemInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		t.Run("unexisting project", func(t *testing.T) {
			sat := planet.Satellites[0]

			service := sat.Admin.Admin.Service
			_, apiErr := service.UpdateProjectLimits(ctx, testrand.UUID(), admin.ProjectLimitsUpdateRequest{})
			require.Error(t, apiErr.Err)
			assert.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("existing project", func(t *testing.T) {
			consoleUser := &console.User{
				ID:               testrand.UUID(),
				FullName:         "Test User",
				Email:            "test@storj.io",
				PasswordHash:     testrand.Bytes(8),
				Status:           console.Inactive,
				UserAgent:        []byte("agent"),
				DefaultPlacement: 5,
			}

			sat := planet.Satellites[0]
			consoleDB := sat.DB.Console()
			_, err := consoleDB.Users().Insert(ctx, consoleUser)
			require.NoError(t, err)

			consoleUser.Status = console.Active
			require.NoError(
				t,
				consoleDB.Users().Update(ctx, consoleUser.ID, console.UpdateUserRequest{Status: &consoleUser.Status}),
			)

			// Project with default limits
			projID := testrand.UUID()
			storage := memory.Size(1000)
			bw := memory.Size(2000)
			segment := int64(10000)
			rate := 500
			burst := 200
			buckets := 500
			consoleProject := &console.Project{
				ID:             projID,
				Name:           "project-free-account",
				Description:    "This is a project created at the time that owner's user account is a free account",
				OwnerID:        consoleUser.ID,
				StorageLimit:   &storage,
				BandwidthLimit: &bw,
				SegmentLimit:   &segment,
				RateLimit:      &rate,
				MaxBuckets:     &buckets,
			}

			consoleProject, err = consoleDB.Projects().Insert(ctx, consoleProject)
			require.NoError(t, err)
			// Insert doesn't set burst limit.
			require.NoError(t, consoleDB.Projects().UpdateBurstLimit(ctx, projID, &burst))

			projPublicID := consoleProject.PublicID

			service := sat.Admin.Admin.Service
			project, apiErr := service.GetProject(ctx, projPublicID)
			require.NoError(t, apiErr.Err)
			require.NotNil(t, project.RateLimit)
			assert.Equal(t, *consoleProject.RateLimit, *project.RateLimit)
			require.NotNil(t, project.BurstLimit)
			assert.Equal(t, burst, *project.BurstLimit)
			require.NotNil(t, project.MaxBuckets)
			assert.Equal(t, *consoleProject.MaxBuckets, *project.MaxBuckets)

			assert.EqualValues(t, consoleProject.BandwidthLimit, project.BandwidthLimit)
			assert.EqualValues(t, consoleProject.StorageLimit, project.StorageLimit)
			assert.EqualValues(t, consoleProject.SegmentLimit, project.SegmentLimit)

			intPtr := func(v int) *int { return &v }
			int64Ptr := func(v int64) *int64 { return &v }

			expectStorage := *project.StorageLimit * 2
			expectBandwidth := *project.BandwidthLimit * 2
			expectSegment := *project.SegmentLimit * 2
			expectBuckets := 100
			expectRate := 2000
			expectBurst := 500
			project, apiErr = service.UpdateProjectLimits(ctx, projPublicID, admin.ProjectLimitsUpdateRequest{
				MaxBuckets:     intPtr(expectBuckets),
				StorageLimit:   int64Ptr(expectStorage),
				BandwidthLimit: int64Ptr(expectBandwidth),
				SegmentLimit:   int64Ptr(expectSegment),
				RateLimit:      intPtr(expectRate),
				BurstLimit:     intPtr(expectBurst),
			})
			require.NoError(t, apiErr.Err)
			require.Equal(t, intPtr(expectBuckets), project.MaxBuckets)
			require.Equal(t, int64Ptr(expectStorage), project.StorageLimit)
			require.Equal(t, int64Ptr(expectBandwidth), project.BandwidthLimit)
			require.Equal(t, int64Ptr(expectSegment), project.SegmentLimit)
			require.Equal(t, intPtr(expectRate), project.RateLimit)
			require.Equal(t, intPtr(expectBurst), project.BurstLimit)
			require.Nil(t, project.UserSetBandwidthLimit)
			require.Nil(t, project.UserSetStorageLimit)
			require.Nil(t, project.RateLimitList)

			// test setting to zero.
			project, apiErr = service.UpdateProjectLimits(ctx, projPublicID, admin.ProjectLimitsUpdateRequest{
				MaxBuckets:     intPtr(0),
				StorageLimit:   int64Ptr(0),
				BandwidthLimit: int64Ptr(0),
				SegmentLimit:   int64Ptr(0),
				RateLimit:      intPtr(0),
				BurstLimit:     intPtr(0),
			})
			require.NoError(t, apiErr.Err)
			require.Equal(t, intPtr(0), project.MaxBuckets)
			require.Equal(t, int64Ptr(0), project.StorageLimit)
			require.Equal(t, int64Ptr(0), project.BandwidthLimit)
			require.Equal(t, int64Ptr(0), project.SegmentLimit)
			require.Equal(t, intPtr(0), project.RateLimit)
			require.Equal(t, intPtr(0), project.BurstLimit)

			// revert
			_, apiErr = service.UpdateProjectLimits(ctx, projPublicID, admin.ProjectLimitsUpdateRequest{
				MaxBuckets:     intPtr(expectBuckets),
				StorageLimit:   int64Ptr(expectStorage),
				BandwidthLimit: int64Ptr(expectBandwidth),
				SegmentLimit:   int64Ptr(expectSegment),
				RateLimit:      intPtr(expectRate),
				BurstLimit:     intPtr(expectBurst),
			})
			require.NoError(t, apiErr.Err)

			// test setting nullable limits to 0 which should make them null in DB
			// first set all nullable fields to non-zero values
			project, apiErr = service.UpdateProjectLimits(ctx, projPublicID, admin.ProjectLimitsUpdateRequest{
				UserSetStorageLimit:   int64Ptr(expectStorage),
				UserSetBandwidthLimit: int64Ptr(expectBandwidth),
				RateLimitHead:         intPtr(expectRate),
				BurstLimitHead:        intPtr(expectBurst),
				RateLimitGet:          intPtr(expectRate),
				BurstLimitGet:         intPtr(expectBurst),
				RateLimitPut:          intPtr(expectRate),
				BurstLimitPut:         intPtr(expectBurst),
				RateLimitDelete:       intPtr(expectRate),
				BurstLimitDelete:      intPtr(expectBurst),
				RateLimitList:         intPtr(expectRate),
				BurstLimitList:        intPtr(expectBurst),
			})
			require.NoError(t, apiErr.Err)
			require.Equal(t, int64Ptr(expectStorage), project.UserSetStorageLimit)
			require.Equal(t, int64Ptr(expectBandwidth), project.UserSetBandwidthLimit)
			require.Equal(t, intPtr(expectRate), project.RateLimitHead)
			require.Equal(t, intPtr(expectBurst), project.BurstLimitHead)
			require.Equal(t, intPtr(expectRate), project.RateLimitGet)
			require.Equal(t, intPtr(expectBurst), project.BurstLimitGet)
			require.Equal(t, intPtr(expectRate), project.RateLimitPut)
			require.Equal(t, intPtr(expectBurst), project.BurstLimitPut)
			require.Equal(t, intPtr(expectRate), project.RateLimitDelete)
			require.Equal(t, intPtr(expectBurst), project.BurstLimitDelete)
			require.Equal(t, intPtr(expectRate), project.RateLimitList)
			require.Equal(t, intPtr(expectBurst), project.BurstLimitList)
			// check that non-nullable values remain unchanged
			require.Equal(t, intPtr(expectBuckets), project.MaxBuckets)
			require.Equal(t, int64Ptr(expectStorage), project.StorageLimit)
			require.Equal(t, int64Ptr(expectBandwidth), project.BandwidthLimit)
			require.Equal(t, int64Ptr(expectSegment), project.SegmentLimit)
			require.Equal(t, intPtr(expectRate), project.RateLimit)
			require.Equal(t, intPtr(expectBurst), project.BurstLimit)

			// now set all nullable fields to 0 to make them null
			project, apiErr = service.UpdateProjectLimits(ctx, projPublicID, admin.ProjectLimitsUpdateRequest{
				UserSetStorageLimit:   int64Ptr(0),
				UserSetBandwidthLimit: int64Ptr(0),
				RateLimitHead:         intPtr(0),
				BurstLimitHead:        intPtr(0),
				RateLimitGet:          intPtr(0),
				BurstLimitGet:         intPtr(0),
				RateLimitPut:          intPtr(0),
				BurstLimitPut:         intPtr(0),
				RateLimitDelete:       intPtr(0),
				BurstLimitDelete:      intPtr(0),
				RateLimitList:         intPtr(0),
				BurstLimitList:        intPtr(0),
			})
			require.NoError(t, apiErr.Err)
			require.Nil(t, project.UserSetStorageLimit)
			require.Nil(t, project.UserSetBandwidthLimit)
			require.Nil(t, project.RateLimitHead)
			require.Nil(t, project.BurstLimitHead)
			require.Nil(t, project.RateLimitGet)
			require.Nil(t, project.BurstLimitGet)
			require.Nil(t, project.RateLimitPut)
			require.Nil(t, project.BurstLimitPut)
			require.Nil(t, project.RateLimitDelete)
			require.Nil(t, project.BurstLimitDelete)
			require.Nil(t, project.RateLimitList)
			require.Nil(t, project.BurstLimitList)
			// check that non-nullable values remain unchanged
			require.Equal(t, intPtr(expectBuckets), project.MaxBuckets)
			require.Equal(t, int64Ptr(expectStorage), project.StorageLimit)
			require.Equal(t, int64Ptr(expectBandwidth), project.BandwidthLimit)
			require.Equal(t, int64Ptr(expectSegment), project.SegmentLimit)
			require.Equal(t, intPtr(expectRate), project.RateLimit)
			require.Equal(t, intPtr(expectBurst), project.BurstLimit)

			_, apiErr = service.UpdateProjectLimits(ctx, projPublicID, admin.ProjectLimitsUpdateRequest{
				MaxBuckets:          intPtr(-1),
				StorageLimit:        int64Ptr(-1),
				UserSetStorageLimit: int64Ptr(-1),
				RateLimit:           intPtr(-1),
				RateLimitList:       intPtr(-1),
			})
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.Error(t, apiErr.Err)
			require.Contains(t, apiErr.Err.Error(), "cannot be negative")
			require.Contains(t, apiErr.Err.Error(), console.BucketsLimit.String())
			require.Contains(t, apiErr.Err.Error(), console.StorageLimit.String())
			require.Contains(t, apiErr.Err.Error(), console.UserSetStorageLimit.String())
			require.Contains(t, apiErr.Err.Error(), console.RateLimitList.String())
			require.Contains(t, apiErr.Err.Error(), console.RateLimit.String())
		})
	})
}
