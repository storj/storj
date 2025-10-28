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
	"storj.io/common/storj"
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
	"storj.io/storj/satellite/nodeselection"
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

			defaultRate := int(sat.Config.Metainfo.RateLimiter.Rate)
			defaultBurst := defaultRate
			defaultMaxBuckets := sat.Config.Metainfo.ProjectLimits.MaxBuckets

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
		authInfo := &admin.AuthInfo{Email: "test@example.com"}

		t.Run("unexisting project", func(t *testing.T) {
			sat := planet.Satellites[0]
			service := sat.Admin.Admin.Service

			_, apiErr := service.UpdateProjectLimits(ctx, authInfo, testrand.UUID(), admin.ProjectLimitsUpdateRequest{})
			assert.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.Error(t, apiErr.Err)
			require.Contains(t, apiErr.Err.Error(), "reason is required")

			_, apiErr = service.UpdateProjectLimits(ctx, authInfo, testrand.UUID(), admin.ProjectLimitsUpdateRequest{
				Reason: "reason",
			})
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
			project, apiErr = service.UpdateProjectLimits(ctx, authInfo, projPublicID, admin.ProjectLimitsUpdateRequest{
				MaxBuckets:     intPtr(expectBuckets),
				StorageLimit:   int64Ptr(expectStorage),
				BandwidthLimit: int64Ptr(expectBandwidth),
				SegmentLimit:   int64Ptr(expectSegment),
				RateLimit:      intPtr(expectRate),
				BurstLimit:     intPtr(expectBurst),
				Reason:         "reason",
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
			project, apiErr = service.UpdateProjectLimits(ctx, authInfo, projPublicID, admin.ProjectLimitsUpdateRequest{
				MaxBuckets:     intPtr(0),
				StorageLimit:   int64Ptr(0),
				BandwidthLimit: int64Ptr(0),
				SegmentLimit:   int64Ptr(0),
				RateLimit:      intPtr(0),
				BurstLimit:     intPtr(0),
				Reason:         "reason",
			})
			require.NoError(t, apiErr.Err)
			require.Equal(t, intPtr(0), project.MaxBuckets)
			require.Equal(t, int64Ptr(0), project.StorageLimit)
			require.Equal(t, int64Ptr(0), project.BandwidthLimit)
			require.Equal(t, int64Ptr(0), project.SegmentLimit)
			require.Equal(t, intPtr(0), project.RateLimit)
			require.Equal(t, intPtr(0), project.BurstLimit)

			// revert
			_, apiErr = service.UpdateProjectLimits(ctx, authInfo, projPublicID, admin.ProjectLimitsUpdateRequest{
				MaxBuckets:     intPtr(expectBuckets),
				StorageLimit:   int64Ptr(expectStorage),
				BandwidthLimit: int64Ptr(expectBandwidth),
				SegmentLimit:   int64Ptr(expectSegment),
				RateLimit:      intPtr(expectRate),
				BurstLimit:     intPtr(expectBurst),
				Reason:         "reason",
			})
			require.NoError(t, apiErr.Err)

			// test setting nullable limits to admin.NullableLimitValue (-1) which should make them null in DB
			// first set all nullable fields to non-zero values
			project, apiErr = service.UpdateProjectLimits(ctx, authInfo, projPublicID, admin.ProjectLimitsUpdateRequest{
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
				Reason:                "reason",
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
			project, apiErr = service.UpdateProjectLimits(ctx, authInfo, projPublicID, admin.ProjectLimitsUpdateRequest{
				UserSetStorageLimit:   int64Ptr(admin.NullableLimitValue),
				UserSetBandwidthLimit: int64Ptr(admin.NullableLimitValue),
				RateLimitHead:         intPtr(admin.NullableLimitValue),
				BurstLimitHead:        intPtr(admin.NullableLimitValue),
				RateLimitGet:          intPtr(admin.NullableLimitValue),
				BurstLimitGet:         intPtr(admin.NullableLimitValue),
				RateLimitPut:          intPtr(admin.NullableLimitValue),
				BurstLimitPut:         intPtr(admin.NullableLimitValue),
				RateLimitDelete:       intPtr(admin.NullableLimitValue),
				BurstLimitDelete:      intPtr(admin.NullableLimitValue),
				RateLimitList:         intPtr(admin.NullableLimitValue),
				BurstLimitList:        intPtr(admin.NullableLimitValue),
				Reason:                "reason",
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

			_, apiErr = service.UpdateProjectLimits(ctx, authInfo, projPublicID, admin.ProjectLimitsUpdateRequest{
				MaxBuckets:          intPtr(-2),
				StorageLimit:        int64Ptr(-1),
				UserSetStorageLimit: int64Ptr(-2),
				RateLimit:           intPtr(-2),
				RateLimitList:       intPtr(-2),
				Reason:              "reason",
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

func TestUpdateProject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{PlacementRules: `0:annotation("location","global");10:annotation("location", "defaultPlacement")`}
				config.Admin.BackOffice.UserGroupsRoleAdmin = []string{"admin"}
				config.Admin.BackOffice.UserGroupsRoleViewer = []string{"viewer"}
				config.Admin.BackOffice.AuditLogger.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		consoleDB := sat.DB.Console()

		// Create a test user
		user, err := sat.AddUser(ctx, console.CreateUser{
			Email:    "test@test.test",
			FullName: "Test User",
		}, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)

		// Create auth info with proper permissions
		authInfo := &admin.AuthInfo{
			Email:  "test@test.test",
			Groups: []string{"admin"},
		}

		t.Run("authentication", func(t *testing.T) {
			newName := "new-project-name"
			req := admin.UpdateProjectRequest{
				Name:   &newName,
				Reason: "testing",
			}
			testFailAuth := func(groups []string) {
				_, apiErr := service.UpdateProject(ctx, &admin.AuthInfo{Groups: groups}, user.ID, req)
				require.True(t, apiErr.Status == http.StatusUnauthorized || apiErr.Status == http.StatusForbidden)
				require.Error(t, apiErr.Err)
				require.Contains(t, apiErr.Err.Error(), "not authorized")
			}

			testFailAuth(nil)
			testFailAuth([]string{})
			testFailAuth([]string{"viewer"}) // insufficient permissions

			_, apiErr := service.UpdateProject(ctx, authInfo, testrand.UUID(), req)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
			require.Error(t, apiErr.Err)
		})

		t.Run("non-existent project", func(t *testing.T) {
			req := admin.UpdateProjectRequest{Reason: "testing"}
			_, apiErr := service.UpdateProject(ctx, authInfo, testrand.UUID(), req)
			require.Error(t, apiErr.Err)
			assert.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("missing reason", func(t *testing.T) {
			newName := "updated-name"
			req := admin.UpdateProjectRequest{Name: &newName}
			_, apiErr := service.UpdateProject(ctx, authInfo, project.PublicID, req)
			require.Error(t, apiErr.Err)
			assert.Equal(t, http.StatusBadRequest, apiErr.Status)
			assert.Contains(t, apiErr.Err.Error(), "reason is required")
		})

		t.Run("update name", func(t *testing.T) {
			newName := "updated-project-name"
			req := admin.UpdateProjectRequest{
				Name:   &newName,
				Reason: "updating project name",
			}
			_, apiErr := service.UpdateProject(ctx, authInfo, project.PublicID, req)
			require.NoError(t, apiErr.Err)

			// Verify the update
			updated, err := consoleDB.Projects().GetByPublicID(ctx, project.PublicID)
			require.NoError(t, err)
			assert.Equal(t, newName, updated.Name)
		})

		t.Run("update description", func(t *testing.T) {
			newDescription := "updated project description"
			req := admin.UpdateProjectRequest{
				Description: &newDescription,
				Reason:      "updating project description",
			}
			_, apiErr := service.UpdateProject(ctx, authInfo, project.PublicID, req)
			require.NoError(t, apiErr.Err)

			// Verify the update
			updated, err := consoleDB.Projects().GetByPublicID(ctx, project.PublicID)
			require.NoError(t, err)
			assert.Equal(t, newDescription, updated.Description)
		})

		t.Run("update user agent", func(t *testing.T) {
			newUserAgent := "new-user-agent"
			req := admin.UpdateProjectRequest{
				UserAgent: &newUserAgent,
				Reason:    "updating user agent",
			}
			_, apiErr := service.UpdateProject(ctx, authInfo, project.PublicID, req)
			require.NoError(t, apiErr.Err)

			// Verify the update
			updated, err := consoleDB.Projects().GetByPublicID(ctx, project.PublicID)
			require.NoError(t, err)
			assert.Equal(t, []byte(newUserAgent), updated.UserAgent)
		})

		t.Run("update status", func(t *testing.T) {
			newStatus := console.ProjectStatus(1)
			req := admin.UpdateProjectRequest{
				Status: &newStatus,
				Reason: "updating project status",
			}
			_, apiErr := service.UpdateProject(ctx, authInfo, project.PublicID, req)
			require.NoError(t, apiErr.Err)

			// Verify the update
			updated, err := consoleDB.Projects().GetByPublicID(ctx, project.PublicID)
			require.NoError(t, err)
			assert.Equal(t, &newStatus, updated.Status)
		})

		t.Run("update default placement", func(t *testing.T) {
			newPlacement := storj.PlacementConstraint(10)
			req := admin.UpdateProjectRequest{
				DefaultPlacement: &newPlacement,
				Reason:           "updating default placement",
			}
			_, apiErr := service.UpdateProject(ctx, authInfo, project.PublicID, req)
			require.NoError(t, apiErr.Err)

			// Verify the update
			updated, err := consoleDB.Projects().GetByPublicID(ctx, project.PublicID)
			require.NoError(t, err)
			assert.Equal(t, newPlacement, updated.DefaultPlacement)
		})

		t.Run("update multiple fields", func(t *testing.T) {
			newName := "multi-update-name"
			newDescription := "multi-update description"
			newUserAgent := "multi-update-agent"
			newPlacement := storj.DefaultPlacement
			req := admin.UpdateProjectRequest{
				Name:             &newName,
				Description:      &newDescription,
				UserAgent:        &newUserAgent,
				DefaultPlacement: &newPlacement,
				Reason:           "updating multiple fields",
			}
			_, apiErr := service.UpdateProject(ctx, authInfo, project.PublicID, req)
			require.NoError(t, apiErr.Err)

			// Verify the updates
			updated, err := consoleDB.Projects().GetByPublicID(ctx, project.PublicID)
			require.NoError(t, err)
			assert.Equal(t, newName, updated.Name)
			assert.Equal(t, newDescription, updated.Description)
			assert.Equal(t, newPlacement, updated.DefaultPlacement)
			assert.Equal(t, []byte(newUserAgent), updated.UserAgent)
		})

		t.Run("empty name validation", func(t *testing.T) {
			emptyName := ""
			req := admin.UpdateProjectRequest{
				Name:   &emptyName,
				Reason: "testing empty name",
			}
			_, apiErr := service.UpdateProject(ctx, authInfo, project.PublicID, req)
			require.Error(t, apiErr.Err)
			assert.Equal(t, http.StatusBadRequest, apiErr.Status)
			assert.Contains(t, apiErr.Err.Error(), "name cannot be empty")
		})

		t.Run("invalid status validation", func(t *testing.T) {
			invalidStatus := console.ProjectStatus(999)
			req := admin.UpdateProjectRequest{
				Status: &invalidStatus,
				Reason: "testing invalid status",
			}
			_, apiErr := service.UpdateProject(ctx, authInfo, project.PublicID, req)
			require.Error(t, apiErr.Err)
			assert.Equal(t, http.StatusBadRequest, apiErr.Status)
			assert.Contains(t, apiErr.Err.Error(), "invalid project status")
		})

		t.Run("invalid placement validation", func(t *testing.T) {
			invalidPlacement := storj.PlacementConstraint(100)
			req := admin.UpdateProjectRequest{
				DefaultPlacement: &invalidPlacement,
				Reason:           "testing invalid placement",
			}
			_, apiErr := service.UpdateProject(ctx, authInfo, project.PublicID, req)
			require.Error(t, apiErr.Err)
			assert.Equal(t, http.StatusBadRequest, apiErr.Status)
			assert.Contains(t, apiErr.Err.Error(), "invalid placement ID")
		})
	})
}
