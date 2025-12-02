// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package auditlogger_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	admin "storj.io/storj/satellite/admin/back-office"
	"storj.io/storj/satellite/admin/back-office/changehistory"
	"storj.io/storj/satellite/console"
)

func TestAuditLogger(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.BackOffice.UserGroupsRoleAdmin = []string{"admin"}
				config.Admin.BackOffice.AuditLogger.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		changeHistoryDB := sat.DB.AdminChangeHistory()

		authInfo := &admin.AuthInfo{Email: "admin@storj.test", Groups: []string{"admin"}}

		t.Run("logs user update to database when enabled", func(t *testing.T) {
			// Create a test user
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test User",
				Email:    "testuser@example.com",
			}, 1)
			require.NoError(t, err)

			// Update the user
			newName := "Updated User"
			updateReq := admin.UpdateUserRequest{
				Name:   &newName,
				Reason: "testing audit logging",
			}

			updatedUser, apiErr := service.UpdateUser(ctx, authInfo, user.ID, updateReq)
			require.Nil(t, apiErr.Err)
			require.NotNil(t, updatedUser)

			// Give the worker time to process the event
			time.Sleep(200 * time.Millisecond)

			// Verify the change was saved to the database
			changes, err := changeHistoryDB.GetChangesByUserID(ctx, user.ID, true)
			require.NoError(t, err)
			require.Len(t, changes, 1)

			change := changes[0]
			require.Equal(t, user.ID, change.UserID)
			require.Nil(t, change.ProjectID)
			require.Nil(t, change.BucketName)
			require.Equal(t, "admin@storj.test", change.AdminEmail)
			require.Equal(t, changehistory.ItemTypeUser, change.ItemType)
			require.Equal(t, "testing audit logging", change.Reason)
			require.Equal(t, "update_user", change.Operation)
			require.NotNil(t, change.Changes)
		})

		t.Run("does not log when disabled", func(t *testing.T) {
			// Disable audit logging
			service.TestToggleAuditLogger(false)

			// Create a test user
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test User 2",
				Email:    "testuser2@example.com",
			}, 1)
			require.NoError(t, err)

			// Update the user
			newName := "Updated User 2"
			updateReq := admin.UpdateUserRequest{
				Name:   &newName,
				Reason: "should not be logged",
			}

			updatedUser, apiErr := service.UpdateUser(ctx, authInfo, user.ID, updateReq)
			require.Nil(t, apiErr.Err)
			require.NotNil(t, updatedUser)

			// Give some time for potential processing
			time.Sleep(200 * time.Millisecond)

			// Verify nothing was saved to the database
			changes, err := changeHistoryDB.GetChangesByUserID(ctx, user.ID, true)
			require.NoError(t, err)
			require.Len(t, changes, 0)
		})
	})
}
