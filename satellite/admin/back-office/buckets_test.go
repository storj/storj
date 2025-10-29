// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	backoffice "storj.io/storj/satellite/admin/back-office"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/satellite/nodeselection"
)

func TestGetProjectBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		bucketsDB := sat.DB.Buckets()

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@test.test",
		}, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)

		now := time.Now()
		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

		_, apiErr := service.GetProjectBuckets(ctx, testrand.UUID(), "", "1", "100", startOfMonth, now)
		require.Error(t, apiErr.Err)
		require.Equal(t, http.StatusNotFound, apiErr.Status)

		bucketsList, apiErr := service.GetProjectBuckets(ctx, project.PublicID, "", "1", "100", startOfMonth, now)
		require.NoError(t, apiErr.Err)
		require.Empty(t, bucketsList.Items)

		// Create test buckets with different configurations
		bucket1 := buckets.Bucket{
			ID:         testrand.UUID(),
			Name:       "bucket1",
			ProjectID:  project.ID,
			Placement:  storj.DefaultPlacement,
			Versioning: buckets.Unversioned,
			UserAgent:  []byte("test-agent-1"),
			ObjectLock: buckets.ObjectLockSettings{
				Enabled: false,
			},
		}

		bucket2 := buckets.Bucket{
			ID:         testrand.UUID(),
			Name:       "bucket2",
			ProjectID:  project.ID,
			Placement:  storj.PlacementConstraint(10),
			Versioning: buckets.VersioningEnabled,
			UserAgent:  []byte("test-agent-2"),
			ObjectLock: buckets.ObjectLockSettings{
				Enabled:              true,
				DefaultRetentionMode: storj.GovernanceMode,
				DefaultRetentionDays: 30,
			},
		}

		_, err = bucketsDB.CreateBucket(ctx, bucket1)
		require.NoError(t, err)

		_, err = bucketsDB.CreateBucket(ctx, bucket2)
		require.NoError(t, err)

		// Get buckets via service
		bucketsList, apiErr = service.GetProjectBuckets(ctx, project.PublicID, "", "1", "100", startOfMonth, now)
		require.NoError(t, apiErr.Err)
		require.Len(t, bucketsList.Items, 2)

		// Verify bucket1
		bucket1Info := findBucketByName(bucketsList.Items, bucket1.Name)
		require.NotNil(t, bucket1Info)
		require.Equal(t, bucket1.Name, bucket1Info.Name)
		require.Equal(t, string(bucket1.UserAgent), bucket1Info.UserAgent)

		// Verify bucket2
		bucket2Info := findBucketByName(bucketsList.Items, bucket2.Name)
		require.NotNil(t, bucket2Info)
		require.Equal(t, bucket2.Name, bucket2Info.Name)
		require.Equal(t, string(bucket2.UserAgent), bucket2Info.UserAgent)
	})
}

// findBucketByName is a helper function to find a bucket by name in the list.
func findBucketByName(buckets []backoffice.BucketInfo, name string) *backoffice.BucketInfo {
	for i := range buckets {
		if buckets[i].Name == name {
			return &buckets[i]
		}
	}
	return nil
}

func TestUpdateBucket(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location", "global");10:annotation("location", "10");11:annotation("location", "11")`,
				}
				config.Admin.BackOffice.UserGroupsRoleAdmin = []string{"admin"}
				config.Admin.BackOffice.UserGroupsRoleViewer = []string{"viewer"}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		bucketsDB := sat.DB.Buckets()

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@test.test",
		}, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)

		authInfo := &backoffice.AuthInfo{
			Email:  "admin@test.test",
			Groups: []string{"admin"},
		}

		// Create test bucket
		bucket := buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "test-bucket",
			ProjectID: project.ID,
			UserAgent: []byte("original-agent"),
		}

		_, err = bucketsDB.CreateBucket(ctx, bucket)
		require.NoError(t, err)

		t.Run("unauthorized", func(t *testing.T) {
			badAuthInfo := &backoffice.AuthInfo{
				Email: "admin@test.test",
			}

			newUserAgent := "new-agent"
			apiErr := service.UpdateBucket(ctx, badAuthInfo, project.PublicID, bucket.Name, backoffice.UpdateBucketRequest{
				UserAgent: &newUserAgent,
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusUnauthorized, apiErr.Status)

			badAuthInfo.Groups = []string{"viewer"}
			apiErr = service.UpdateBucket(ctx, badAuthInfo, project.PublicID, bucket.Name, backoffice.UpdateBucketRequest{
				UserAgent: &newUserAgent,
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusForbidden, apiErr.Status)
		})

		t.Run("missing reason", func(t *testing.T) {
			newUserAgent := "new-agent"
			apiErr := service.UpdateBucket(ctx, authInfo, project.PublicID, bucket.Name, backoffice.UpdateBucketRequest{
				UserAgent: &newUserAgent,
				Reason:    "",
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.Contains(t, apiErr.Err.Error(), "reason is required")
		})

		t.Run("invalid placement", func(t *testing.T) {
			invalidPlacement := storj.PlacementConstraint(20)
			apiErr := service.UpdateBucket(ctx, authInfo, project.PublicID, bucket.Name, backoffice.UpdateBucketRequest{
				Placement: &invalidPlacement,
				Reason:    "test invalid placement",
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.Contains(t, apiErr.Err.Error(), "invalid placement")
		})

		t.Run("project not found", func(t *testing.T) {
			newUserAgent := "new-agent"
			apiErr := service.UpdateBucket(ctx, authInfo, testrand.UUID(), bucket.Name, backoffice.UpdateBucketRequest{
				UserAgent: &newUserAgent,
				Reason:    "test project not found",
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
			require.Contains(t, apiErr.Err.Error(), "project not found")
		})

		t.Run("bucket not found", func(t *testing.T) {
			newUserAgent := "new-agent"
			apiErr := service.UpdateBucket(ctx, authInfo, project.PublicID, "nonexistent-bucket", backoffice.UpdateBucketRequest{
				UserAgent: &newUserAgent,
				Reason:    "test bucket not found",
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
			require.Contains(t, apiErr.Err.Error(), "bucket not found")
		})

		t.Run("update successfully", func(t *testing.T) {
			newUserAgent := "updated-agent"
			newPlacement := storj.PlacementConstraint(10)
			apiErr := service.UpdateBucket(ctx, authInfo, project.PublicID, bucket.Name, backoffice.UpdateBucketRequest{
				UserAgent: &newUserAgent,
				Placement: &newPlacement,
				Reason:    "testing update",
			})
			require.NoError(t, apiErr.Err)

			// Verify the update
			updatedBucket, err := bucketsDB.GetBucket(ctx, []byte(bucket.Name), project.ID)
			require.NoError(t, err)
			require.Equal(t, newUserAgent, string(updatedBucket.UserAgent))
			require.Equal(t, newPlacement, updatedBucket.Placement)
		})

		t.Run("update placement unsuccessfully on non-empty bucket", func(t *testing.T) {
			// confirm the bucket is initially empty
			state, apiError := service.GetBucketState(ctx, project.PublicID, bucket.Name)
			require.NoError(t, apiError.Err)
			require.NotNil(t, state)
			require.True(t, state.Empty)

			// add objects to the bucket to make it non-empty
			err = sat.Metabase.DB.TestingBatchInsertObjects(ctx, []metabase.RawObject{
				{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  project.ID,
						BucketName: metabase.BucketName(bucket.Name),
						ObjectKey:  "key",
						Version:    1,
						StreamID:   uuid.UUID{},
					},
					EncryptedUserData: metabasetest.RandEncryptedUserData(),
					Status:            metabase.CommittedUnversioned,
				},
			})
			require.NoError(t, err)

			// verify the bucket is non-empty
			state, apiError = service.GetBucketState(ctx, project.PublicID, bucket.Name)
			require.NoError(t, apiError.Err)
			require.NotNil(t, state)
			require.False(t, state.Empty)

			// try to update the placement
			newPlacement := storj.PlacementConstraint(11)
			apiErr := service.UpdateBucket(ctx, authInfo, project.PublicID, bucket.Name, backoffice.UpdateBucketRequest{
				Placement: &newPlacement,
				Reason:    "testing update on non-empty bucket",
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusConflict, apiErr.Status)
			require.Contains(t, apiErr.Err.Error(), "cannot change placement of non-empty bucket")
		})
	})
}
