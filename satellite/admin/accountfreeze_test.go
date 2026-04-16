// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	backoffice "storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/console"
)

func TestFreezeUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.LiveAccounting.AsOfSystemInterval = 0
				config.Admin.UserGroupsRoleAdmin = []string{"admin"}
				config.Admin.UserGroupsRoleViewer = []string{"viewer"}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service

		for _, unauthorizedInfo := range []*backoffice.AuthInfo{nil, {}} {
			apiErr := service.ToggleFreezeUser(ctx, unauthorizedInfo, uuid.UUID{}, backoffice.ToggleFreezeUserRequest{})
			require.Equal(t, http.StatusUnauthorized, apiErr.Status)
		}

		request := backoffice.ToggleFreezeUserRequest{Reason: "reason", Action: backoffice.FreezeActionUnfreeze}

		apiErr := service.ToggleFreezeUser(ctx, &backoffice.AuthInfo{Groups: []string{"somerole"}}, uuid.UUID{}, request)
		require.Equal(t, http.StatusForbidden, apiErr.Status)

		apiErr = service.ToggleFreezeUser(ctx, &backoffice.AuthInfo{Groups: []string{"viewer"}}, uuid.UUID{}, request)
		require.Equal(t, http.StatusForbidden, apiErr.Status)

		authInfo := &backoffice.AuthInfo{Groups: []string{"admin"}}

		request.Reason = ""
		apiErr = service.ToggleFreezeUser(ctx, authInfo, uuid.UUID{}, request)
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "reason is required")

		request.Reason = "reason"
		request.Action = ""
		apiErr = service.ToggleFreezeUser(ctx, authInfo, uuid.UUID{}, request)
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "invalid action")

		request.Action = backoffice.FreezeActionFreeze
		apiErr = service.ToggleFreezeUser(ctx, authInfo, uuid.UUID{}, request)
		require.Equal(t, http.StatusNotFound, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "user not found")

		request.Type = console.BillingWarning
		apiErr = service.ToggleFreezeUser(ctx, authInfo, uuid.UUID{}, request)
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "unsupported freeze event type")

		user, err := sat.AddUser(ctx, console.CreateUser{
			Email:    "test@test.test",
			FullName: "Test User",
		}, 0)
		require.NoError(t, err)

		types, apiErr := service.GetFreezeEventTypes(ctx)
		require.NoError(t, apiErr.Err)
		require.NotEmpty(t, types)

		for _, eventType := range types {
			request.Type = eventType.Value
			apiErr = service.ToggleFreezeUser(ctx, authInfo, user.ID, request)
			require.NoError(t, apiErr.Err)

			account, apiErr := service.GetUserByEmail(ctx, user.Email)
			require.NoError(t, apiErr.Err)
			require.NotNil(t, account.FreezeStatus)
			require.Equal(t, eventType.Value, account.FreezeStatus.Value)

			request.Action = backoffice.FreezeActionUnfreeze
			apiErr = service.ToggleFreezeUser(ctx, authInfo, user.ID, request)
			require.NoError(t, apiErr.Err)

			account, apiErr = service.GetUserByEmail(ctx, user.Email)
			require.NoError(t, apiErr.Err)
			require.Nil(t, account.FreezeStatus)

			request.Action = backoffice.FreezeActionFreeze
			request.Type = eventType.Value
		}
	})
}

func TestToggleFreezeUserTenantScoping(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.UserGroupsRoleAdmin = []string{"admin"}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		consoleDB := sat.DB.Console()

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
			ID: testrand.UUID(), FullName: "A", Email: "freeze-a@example.com",
			PasswordHash: make([]byte, 0), TenantID: &tenantA,
		})
		userB := insertActive(&console.User{
			ID: testrand.UUID(), FullName: "B", Email: "freeze-b@example.com",
			PasswordHash: make([]byte, 0), TenantID: &tenantB,
		})

		authInfo := &backoffice.AuthInfo{Groups: []string{"admin"}, Email: "admin@example.com"}
		freezeReq := backoffice.ToggleFreezeUserRequest{
			Action: backoffice.FreezeActionFreeze,
			Type:   console.BillingFreeze,
			Reason: "test",
		}

		t.Run("general admin can freeze any user", func(t *testing.T) {
			service.TestSetTenantID(nil)

			apiErr := service.ToggleFreezeUser(ctx, authInfo, userA.ID, freezeReq)
			require.NoError(t, apiErr.Err)

			// unfreeze to reset state
			unfreeze := backoffice.ToggleFreezeUserRequest{Action: backoffice.FreezeActionUnfreeze, Reason: "reset"}
			apiErr = service.ToggleFreezeUser(ctx, authInfo, userA.ID, unfreeze)
			require.NoError(t, apiErr.Err)
		})

		t.Run("tenant-scoped admin can freeze own tenant user", func(t *testing.T) {
			service.TestSetTenantID(&tenantA)

			apiErr := service.ToggleFreezeUser(ctx, authInfo, userA.ID, freezeReq)
			require.NoError(t, apiErr.Err)

			// unfreeze to reset state
			unfreeze := backoffice.ToggleFreezeUserRequest{Action: backoffice.FreezeActionUnfreeze, Reason: "reset"}
			apiErr = service.ToggleFreezeUser(ctx, authInfo, userA.ID, unfreeze)
			require.NoError(t, apiErr.Err)
		})

		t.Run("tenant-scoped admin cannot freeze other tenant user", func(t *testing.T) {
			service.TestSetTenantID(&tenantA)

			apiErr := service.ToggleFreezeUser(ctx, authInfo, userB.ID, freezeReq)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("tenant-scoped admin cannot unfreeze other tenant user", func(t *testing.T) {
			// freeze userB as general admin first
			service.TestSetTenantID(nil)
			apiErr := service.ToggleFreezeUser(ctx, authInfo, userB.ID, freezeReq)
			require.NoError(t, apiErr.Err)

			// attempt unfreeze as tenant-A admin
			service.TestSetTenantID(&tenantA)
			unfreeze := backoffice.ToggleFreezeUserRequest{Action: backoffice.FreezeActionUnfreeze, Reason: "test"}
			apiErr = service.ToggleFreezeUser(ctx, authInfo, userB.ID, unfreeze)
			require.Equal(t, http.StatusNotFound, apiErr.Status)

			// clean up
			service.TestSetTenantID(nil)
			apiErr = service.ToggleFreezeUser(ctx, authInfo, userB.ID, unfreeze)
			require.NoError(t, apiErr.Err)
		})

		t.Run("tenant-scoped admin gets 404 for unknown user ID", func(t *testing.T) {
			service.TestSetTenantID(&tenantA)

			apiErr := service.ToggleFreezeUser(ctx, authInfo, uuid.UUID{}, freezeReq)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("HideFreezeActions blocks freeze and unfreeze", func(t *testing.T) {
			service.TestSetTenantID(nil)
			service.TestSetHideFreezeActions(true)
			defer service.TestSetHideFreezeActions(false)

			apiErr := service.ToggleFreezeUser(ctx, authInfo, userA.ID, freezeReq)
			require.Equal(t, http.StatusForbidden, apiErr.Status)

			unfreeze := backoffice.ToggleFreezeUserRequest{Action: backoffice.FreezeActionUnfreeze, Reason: "test"}
			apiErr = service.ToggleFreezeUser(ctx, authInfo, userA.ID, unfreeze)
			require.Equal(t, http.StatusForbidden, apiErr.Status)
		})
	})
}
