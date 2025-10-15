// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	backoffice "storj.io/storj/satellite/admin/back-office"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
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
