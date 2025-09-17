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
	admin "storj.io/storj/satellite/admin/back-office"
	"storj.io/storj/satellite/console"
)

func TestFreezeUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.LiveAccounting.AsOfSystemInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service

		apiErr := service.FreezeUser(ctx, uuid.UUID{}, admin.FreezeUserRequest{
			Type: console.BillingFreeze,
		})
		require.Equal(t, http.StatusNotFound, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "user not found")

		apiErr = service.FreezeUser(ctx, uuid.UUID{}, admin.FreezeUserRequest{
			Type: console.BillingWarning,
		})
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
			apiErr = service.FreezeUser(ctx, user.ID, admin.FreezeUserRequest{Type: eventType.Value})
			require.NoError(t, apiErr.Err)

			account, apiErr := service.GetUserByEmail(ctx, user.Email)
			require.NoError(t, apiErr.Err)
			require.NotNil(t, account.FreezeStatus)
			require.Equal(t, eventType.Value, account.FreezeStatus.Value)

			apiErr = service.UnfreezeUser(ctx, user.ID)
			require.NoError(t, apiErr.Err)

			account, apiErr = service.GetUserByEmail(ctx, user.Email)
			require.NoError(t, apiErr.Err)
			require.Nil(t, account.FreezeStatus)
		}

		apiErr = service.UnfreezeUser(ctx, uuid.UUID{})
		require.Equal(t, http.StatusNotFound, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "user not found")
	})
}
