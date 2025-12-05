// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	backoffice "storj.io/storj/satellite/admin/back-office"
	"storj.io/storj/satellite/console"
)

func TestFreezeUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.LiveAccounting.AsOfSystemInterval = 0
				config.Admin.BackOffice.UserGroupsRoleAdmin = []string{"admin"}
				config.Admin.BackOffice.UserGroupsRoleViewer = []string{"viewer"}
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
