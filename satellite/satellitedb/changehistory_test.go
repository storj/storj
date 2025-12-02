// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/admin/back-office/changehistory"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestChangeHistories(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		changeHistories := db.AdminChangeHistory()

		userID := testrand.UUID()
		projectID := testrand.UUID()
		bucketName := testrand.BucketName()
		timestamp := time.Now().Truncate(time.Second)

		// Log a user change
		_, err := changeHistories.LogChange(ctx, changehistory.ChangeLog{
			UserID:     userID,
			AdminEmail: "admin@example.com",
			ItemType:   changehistory.ItemTypeUser,
			Operation:  "update_user",
			Reason:     "test",
			Changes:    map[string]any{"field": "value"},
			Timestamp:  timestamp,
		})
		require.NoError(t, err)

		// Log a project change for the same user
		_, err = changeHistories.LogChange(ctx, changehistory.ChangeLog{
			UserID:     userID,
			ProjectID:  &projectID,
			AdminEmail: "admin@example.com",
			ItemType:   changehistory.ItemTypeProject,
			Operation:  "update_project",
			Reason:     "test",
			Changes:    map[string]any{"field": "value"},
			Timestamp:  timestamp,
		})
		require.NoError(t, err)

		// Log a bucket change for the same user
		_, err = changeHistories.LogChange(ctx, changehistory.ChangeLog{
			UserID:     userID,
			ProjectID:  &projectID,
			BucketName: &bucketName,
			AdminEmail: "admin@example.com",
			ItemType:   changehistory.ItemTypeBucket,
			Operation:  "update_bucket",
			Reason:     "test",
			Changes:    map[string]any{"field": "value"},
			Timestamp:  timestamp,
		})
		require.NoError(t, err)

		t.Run("GetChangesByUserID", func(t *testing.T) {
			// Get changes with exact=true (should only return user changes)
			exactChanges, err := changeHistories.GetChangesByUserID(ctx, userID, true)
			require.NoError(t, err)
			require.Len(t, exactChanges, 1)
			require.Equal(t, changehistory.ItemTypeUser, exactChanges[0].ItemType)

			// verify the exact change details
			require.Equal(t, "update_user", exactChanges[0].Operation)
			require.Equal(t, "test", exactChanges[0].Reason)
			require.Len(t, exactChanges[0].Changes, 1)
			require.Equal(t, "value", exactChanges[0].Changes["field"])
			require.True(t, timestamp.Equal(exactChanges[0].Timestamp))

			// Get changes with exact=false (should return all changes)
			allChanges, err := changeHistories.GetChangesByUserID(ctx, userID, false)
			require.NoError(t, err)
			require.Len(t, allChanges, 3)
		})

		t.Run("GetChangesByProjectID", func(t *testing.T) {
			// Get changes with exact=true (should only return project changes)
			exactChanges, err := changeHistories.GetChangesByProjectID(ctx, projectID, true)
			require.NoError(t, err)
			require.Len(t, exactChanges, 1)
			require.Equal(t, changehistory.ItemTypeProject, exactChanges[0].ItemType)

			// Get changes with exact=false (should return both project and bucket changes)
			allChanges, err := changeHistories.GetChangesByProjectID(ctx, projectID, false)
			require.NoError(t, err)
			require.Len(t, allChanges, 2)
		})

		t.Run("GetChangesByBucketName", func(t *testing.T) {
			changes, err := changeHistories.GetChangesByBucketName(ctx, bucketName)
			require.NoError(t, err)
			require.Len(t, changes, 1)
			require.Equal(t, changehistory.ItemTypeBucket, changes[0].ItemType)
		})
	})
}
