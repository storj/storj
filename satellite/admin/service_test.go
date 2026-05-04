// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	backoffice "storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/admin/changehistory"
	"storj.io/storj/satellite/console"
)

func TestSearchUsersOrProjects(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		consoleDB := sat.DB.Console()

		consoleUser, err := consoleDB.Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "Test User",
			Email:        "test@storj.io",
			Status:       console.Active,
			PasswordHash: make([]byte, 0),
		})
		require.NoError(t, err)
		require.NoError(t, sat.DB.StripeCoinPayments().Customers().Insert(ctx, consoleUser.ID, "cus_random_customer_id"))

		project, err := sat.AddProject(ctx, consoleUser.ID, "Test Project 1")
		require.NoError(t, err)
		require.NotNil(t, project.Status)

		for _, unauthorizedInfo := range []*backoffice.AuthInfo{nil, {}} {
			_, apiErr := service.SearchUsersProjectsOrNodes(ctx, unauthorizedInfo, "")
			require.Equal(t, http.StatusUnauthorized, apiErr.Status)
		}

		_, apiErr := service.SearchUsersProjectsOrNodes(ctx, &backoffice.AuthInfo{Groups: []string{"somerole"}}, "")
		require.Equal(t, http.StatusForbidden, apiErr.Status)

		service.TestSetRoleViewer("viewer")
		authInfo := &backoffice.AuthInfo{Groups: []string{"viewer"}}

		confirmUser := func(result *backoffice.SearchResult) {
			require.NotNil(t, result)
			user := result.Accounts[0]
			require.Equal(t, consoleUser.ID, user.ID)
			require.Equal(t, consoleUser.Email, user.Email)
			require.Equal(t, consoleUser.FullName, user.FullName)
			require.Equal(t, consoleUser.Status.Info(), user.Status)
			require.Equal(t, consoleUser.Kind.Info(), user.Kind)
		}

		result, apiErr := service.SearchUsersProjectsOrNodes(ctx, authInfo, "nothing")
		require.NoError(t, apiErr.Err)
		require.NotNil(t, result)
		require.Nil(t, result.Project)
		require.Empty(t, result.Accounts)

		customerID, err := consoleDB.Users().GetCustomerID(ctx, consoleUser.ID)
		require.NoError(t, err)
		require.NotEmpty(t, customerID)

		noDashes := func(id uuid.UUID) string { return strings.ReplaceAll(id.String(), "-", "") }

		for _, term := range []string{consoleUser.Email, "test@", "User", consoleUser.ID.String(), noDashes(consoleUser.ID), customerID} {
			result, apiErr = service.SearchUsersProjectsOrNodes(ctx, authInfo, term)
			require.NoError(t, apiErr.Err)
			require.Len(t, result.Accounts, 1)
			confirmUser(result)
		}

		for _, term := range []string{project.ID.String(), noDashes(project.ID), project.PublicID.String(), noDashes(project.PublicID)} {
			result, apiErr = service.SearchUsersProjectsOrNodes(ctx, authInfo, term)
			require.NoError(t, apiErr.Err)
			require.NotNil(t, result)
			p := result.Project
			require.NotNil(t, p)
			require.Equal(t, project.ID, p.ID)
			require.Equal(t, project.PublicID, p.PublicID)
			require.Equal(t, project.Name, p.Name)
			require.Equal(t, consoleUser.ID, p.Owner.ID)
		}

		// searching by invalid ID should return no results
		result, apiErr = service.SearchUsersProjectsOrNodes(ctx, authInfo, uuid.UUID{}.String())
		require.NoError(t, apiErr.Err)
		require.NotNil(t, result)
		require.Nil(t, result.Project)
		require.Empty(t, result.Accounts)

		// unknown customer ID returns no results
		result, apiErr = service.SearchUsersProjectsOrNodes(ctx, authInfo, customerID+"who")
		require.NoError(t, apiErr.Err)
		require.NotNil(t, result)
		require.Nil(t, result.Project)
		require.Empty(t, result.Accounts)

		_, apiErr = service.SearchUsersProjectsOrNodes(ctx, authInfo, "")
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)
	})
}

func TestGetChangeHistoryTenantScoping(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		consoleDB := sat.DB.Console()
		changeHistoryDB := sat.DB.AdminChangeHistory()

		tenantA := "tenant-a"
		tenantB := "tenant-b"
		activeStatus := console.Active

		insertActive := func(u *console.User) *console.User {
			inserted, err := consoleDB.Users().Insert(ctx, u)
			require.NoError(t, err)
			require.NoError(t, consoleDB.Users().Update(ctx, inserted.ID, console.UpdateUserRequest{Status: &activeStatus}))
			inserted.Status = activeStatus
			return inserted
		}

		userA := insertActive(&console.User{
			ID: testrand.UUID(), FullName: "A", Email: "ch-a@example.com",
			PasswordHash: make([]byte, 0), TenantID: &tenantA,
		})
		userB := insertActive(&console.User{
			ID: testrand.UUID(), FullName: "B", Email: "ch-b@example.com",
			PasswordHash: make([]byte, 0), TenantID: &tenantB,
		})

		projectA, err := consoleDB.Projects().Insert(ctx, &console.Project{
			ID: testrand.UUID(), Name: "proj-a", OwnerID: userA.ID,
		})
		require.NoError(t, err)
		projectB, err := consoleDB.Projects().Insert(ctx, &console.Project{
			ID: testrand.UUID(), Name: "proj-b", OwnerID: userB.ID,
		})
		require.NoError(t, err)

		bucketName := testrand.BucketName()
		timestamp := time.Now().Truncate(time.Second)

		_, err = changeHistoryDB.LogChange(ctx, changehistory.ChangeLog{
			UserID: userA.ID, AdminEmail: "admin@example.com",
			ItemType: changehistory.ItemTypeUser, Operation: "update_user",
			Reason: "r", Changes: map[string]any{}, Timestamp: timestamp,
		})
		require.NoError(t, err)
		_, err = changeHistoryDB.LogChange(ctx, changehistory.ChangeLog{
			UserID: userA.ID, ProjectID: &projectA.PublicID, AdminEmail: "admin@example.com",
			ItemType: changehistory.ItemTypeProject, Operation: "update_project",
			Reason: "r", Changes: map[string]any{}, Timestamp: timestamp,
		})
		require.NoError(t, err)
		_, err = changeHistoryDB.LogChange(ctx, changehistory.ChangeLog{
			UserID: userA.ID, ProjectID: &projectA.PublicID, BucketName: &bucketName,
			AdminEmail: "admin@example.com",
			ItemType:   changehistory.ItemTypeBucket, Operation: "update_bucket",
			Reason: "r", Changes: map[string]any{}, Timestamp: timestamp,
		})
		require.NoError(t, err)

		t.Run("general admin: user and project history accessible", func(t *testing.T) {
			service.TestSetTenantID(nil)

			_, apiErr := service.GetChangeHistory(ctx, "false", string(changehistory.ItemTypeUser), userA.ID.String())
			require.NoError(t, apiErr.Err)

			_, apiErr = service.GetChangeHistory(ctx, "false", string(changehistory.ItemTypeProject), projectA.PublicID.String())
			require.NoError(t, apiErr.Err)

			_, apiErr = service.GetChangeHistory(ctx, "false", string(changehistory.ItemTypeBucket), bucketName)
			require.NoError(t, apiErr.Err)
		})

		t.Run("tenant-scoped admin: own tenant accessible", func(t *testing.T) {
			service.TestSetTenantID(&tenantA)

			_, apiErr := service.GetChangeHistory(ctx, "false", string(changehistory.ItemTypeUser), userA.ID.String())
			require.NoError(t, apiErr.Err)

			_, apiErr = service.GetChangeHistory(ctx, "false", string(changehistory.ItemTypeProject), projectA.PublicID.String())
			require.NoError(t, apiErr.Err)
		})

		t.Run("tenant-scoped admin: other tenant returns 404", func(t *testing.T) {
			service.TestSetTenantID(&tenantA)

			_, apiErr := service.GetChangeHistory(ctx, "false", string(changehistory.ItemTypeUser), userB.ID.String())
			require.Equal(t, http.StatusNotFound, apiErr.Status)

			_, apiErr = service.GetChangeHistory(ctx, "false", string(changehistory.ItemTypeProject), projectB.PublicID.String())
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("tenant-scoped admin: bucket-by-name returns 403", func(t *testing.T) {
			service.TestSetTenantID(&tenantA)

			_, apiErr := service.GetChangeHistory(ctx, "false", string(changehistory.ItemTypeBucket), bucketName)
			require.Equal(t, http.StatusForbidden, apiErr.Status)
		})
	})
}

func TestNodesTenantScoping(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service

		tenantA := "tenant-a"
		nodeID := planet.StorageNodes[0].ID().String()

		t.Run("general admin can get node info", func(t *testing.T) {
			service.TestSetTenantID(nil)

			_, apiErr := service.GetNodeInfo(ctx, nodeID)
			require.NoError(t, apiErr.Err)
		})

		t.Run("tenant-scoped admin gets 403 on GetNodeInfo", func(t *testing.T) {
			service.TestSetTenantID(&tenantA)

			_, apiErr := service.GetNodeInfo(ctx, nodeID)
			require.Equal(t, http.StatusForbidden, apiErr.Status)
		})
	})
}

func TestGetChangeHistory(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		consoleDB := sat.DB.Console()
		changeHistoryDB := sat.DB.AdminChangeHistory()

		// Create a real user and project so GetChangeHistory can validate them.
		activeStatus := console.Active
		user, err := consoleDB.Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "History User",
			Email:        "history@example.com",
			PasswordHash: make([]byte, 0),
		})
		require.NoError(t, err)
		require.NoError(t, consoleDB.Users().Update(ctx, user.ID, console.UpdateUserRequest{Status: &activeStatus}))

		project, err := consoleDB.Projects().Insert(ctx, &console.Project{
			ID:      testrand.UUID(),
			Name:    "history-project",
			OwnerID: user.ID,
		})
		require.NoError(t, err)

		userID := user.ID
		projectID := project.PublicID
		bucketName := testrand.BucketName()
		timestamp := time.Now().Truncate(time.Second)

		// Log a user change
		_, err = changeHistoryDB.LogChange(ctx, changehistory.ChangeLog{
			UserID:     userID,
			AdminEmail: "admin@example.com",
			ItemType:   changehistory.ItemTypeUser,
			Operation:  "update_user",
			Reason:     "test user change",
			Changes:    map[string]any{"field": "user_value"},
			Timestamp:  timestamp,
		})
		require.NoError(t, err)

		// Log a project change for the same user
		_, err = changeHistoryDB.LogChange(ctx, changehistory.ChangeLog{
			UserID:     userID,
			ProjectID:  &projectID,
			AdminEmail: "admin@example.com",
			ItemType:   changehistory.ItemTypeProject,
			Operation:  "update_project",
			Reason:     "test project change",
			Changes:    map[string]any{"field": "project_value"},
			Timestamp:  timestamp,
		})
		require.NoError(t, err)

		// Log a bucket change for the same user
		_, err = changeHistoryDB.LogChange(ctx, changehistory.ChangeLog{
			UserID:     userID,
			ProjectID:  &projectID,
			BucketName: &bucketName,
			AdminEmail: "admin@example.com",
			ItemType:   changehistory.ItemTypeBucket,
			Operation:  "update_bucket",
			Reason:     "test bucket change",
			Changes:    map[string]any{"field": "bucket_value"},
			Timestamp:  timestamp,
		})
		require.NoError(t, err)

		t.Run("get changes by userID with exact=true", func(t *testing.T) {
			changes, apiErr := service.GetChangeHistory(ctx, "true", string(changehistory.ItemTypeUser), userID.String())
			require.Nil(t, apiErr.Err)
			require.Len(t, changes, 1)
			require.Equal(t, changehistory.ItemTypeUser, changes[0].ItemType)
			require.Equal(t, "update_user", changes[0].Operation)
		})

		t.Run("get changes by userID with exact=false", func(t *testing.T) {
			changes, apiErr := service.GetChangeHistory(ctx, "false", string(changehistory.ItemTypeUser), userID.String())
			require.Nil(t, apiErr.Err)
			require.Len(t, changes, 3)
		})

		t.Run("get changes by projectID with exact=true", func(t *testing.T) {
			changes, apiErr := service.GetChangeHistory(ctx, "true", string(changehistory.ItemTypeProject), projectID.String())
			require.Nil(t, apiErr.Err)
			require.Len(t, changes, 1)
			require.Equal(t, changehistory.ItemTypeProject, changes[0].ItemType)
			require.Equal(t, "update_project", changes[0].Operation)
		})

		t.Run("get changes by projectID with exact=false", func(t *testing.T) {
			changes, apiErr := service.GetChangeHistory(ctx, "false", string(changehistory.ItemTypeProject), projectID.String())
			require.Nil(t, apiErr.Err)
			require.Len(t, changes, 2)
		})

		t.Run("get changes by bucketID", func(t *testing.T) {
			changes, apiErr := service.GetChangeHistory(ctx, "false", string(changehistory.ItemTypeBucket), bucketName)
			require.Nil(t, apiErr.Err)
			require.Len(t, changes, 1)
			require.Equal(t, changehistory.ItemTypeBucket, changes[0].ItemType)
			require.Equal(t, "update_bucket", changes[0].Operation)
		})

		t.Run("returns bad request for unknown item type", func(t *testing.T) {
			changes, apiErr := service.GetChangeHistory(ctx, "false", "UnknownType", userID.String())
			require.Nil(t, changes)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.Error(t, apiErr.Err)
		})
	})
}
