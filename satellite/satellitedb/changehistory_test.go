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
		timestamp := time.Now().Truncate(time.Microsecond)

		params := changehistory.ChangeLog{
			UserID:     userID,
			ProjectID:  &projectID,
			BucketName: &bucketName,
			AdminEmail: "admin@example.com",
			ItemType:   changehistory.ItemTypeBucket,
			Reason:     "user request",
			Operation:  "update",
			Changes: map[string]any{
				"field1": "value1",
				"field2": 42,
				"field3": true,
			},
			Timestamp: timestamp,
		}

		result, err := changeHistories.LogChange(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify all fields match
		require.Equal(t, userID, result.UserID)
		require.NotNil(t, result.ProjectID)
		require.Equal(t, projectID, *result.ProjectID)
		require.NotNil(t, result.BucketName)
		require.Equal(t, bucketName, *result.BucketName)
		require.Equal(t, "admin@example.com", result.AdminEmail)
		require.Equal(t, changehistory.ItemTypeBucket, result.ItemType)
		require.Equal(t, "user request", result.Reason)
		require.Equal(t, "update", result.Operation)
		require.True(t, timestamp.Equal(result.Timestamp))

		// Verify changes map
		require.Len(t, result.Changes, 3)
		require.Equal(t, "value1", result.Changes["field1"])
		require.Equal(t, float64(42), result.Changes["field2"]) // JSON unmarshal converts numbers to float64
		require.Equal(t, true, result.Changes["field3"])

		// Verify we can retrieve the change by user ID
		userChanges, err := changeHistories.TestListChangesByUserID(ctx, userID)
		require.NoError(t, err)
		require.Len(t, userChanges, 1)
		require.Equal(t, result.UserID, userChanges[0].UserID)
		require.Equal(t, result.AdminEmail, userChanges[0].AdminEmail)
		require.Equal(t, result.ItemType, userChanges[0].ItemType)
		require.Equal(t, result.Reason, userChanges[0].Reason)
		require.Equal(t, result.Operation, userChanges[0].Operation)
		require.Len(t, userChanges[0].Changes, len(result.Changes))
	})
}
