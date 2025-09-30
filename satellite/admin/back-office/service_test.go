// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	backoffice "storj.io/storj/satellite/admin/back-office"
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
			_, apiErr := service.SearchUsersOrProjects(ctx, unauthorizedInfo, "")
			require.Equal(t, http.StatusUnauthorized, apiErr.Status)
		}

		_, apiErr := service.SearchUsersOrProjects(ctx, &backoffice.AuthInfo{Groups: []string{"somerole"}}, "")
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

		result, apiErr := service.SearchUsersOrProjects(ctx, authInfo, "nothing")
		require.NoError(t, apiErr.Err)
		require.NotNil(t, result)
		require.Nil(t, result.Project)
		require.Empty(t, result.Accounts)

		customerID, err := consoleDB.Users().GetCustomerID(ctx, consoleUser.ID)
		require.NoError(t, err)
		require.NotEmpty(t, customerID)

		for _, term := range []string{consoleUser.Email, "test@", "User", consoleUser.ID.String(), customerID} {
			result, apiErr = service.SearchUsersOrProjects(ctx, authInfo, term)
			require.NoError(t, apiErr.Err)
			require.Len(t, result.Accounts, 1)
			confirmUser(result)
		}

		for _, id := range []uuid.UUID{project.ID, project.PublicID} {
			result, apiErr = service.SearchUsersOrProjects(ctx, authInfo, id.String())
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
		result, apiErr = service.SearchUsersOrProjects(ctx, authInfo, uuid.UUID{}.String())
		require.NoError(t, apiErr.Err)
		require.NotNil(t, result)
		require.Nil(t, result.Project)
		require.Empty(t, result.Accounts)

		// unknown customer ID returns no results
		result, apiErr = service.SearchUsersOrProjects(ctx, authInfo, customerID+"who")
		require.NoError(t, apiErr.Err)
		require.NotNil(t, result)
		require.Nil(t, result.Project)
		require.Empty(t, result.Accounts)

		_, apiErr = service.SearchUsersOrProjects(ctx, authInfo, "")
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)
	})
}
