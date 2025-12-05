// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestABMethodsOnError(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.ABTesting.Enabled = true
				config.Console.ABTesting.APIKey = "APIKey"
				config.Console.ABTesting.EnvId = "EnvId"
				config.Console.ABTesting.FlagshipURL = "FlagshipURL"
				config.Console.ABTesting.HitTrackingURL = "HitTrackingURL"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.ABTesting.Service

		newUser := console.CreateUser{
			FullName:  "AB-Tester",
			ShortName: "",
			Email:     "ab@test.test",
		}

		user, err := sat.AddUser(ctx, newUser, 1)
		require.NoError(t, err)

		_, status, err := doRequestWithAuth(ctx, t, sat, user, http.MethodGet, "ab/values", nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, status)

		values, err := service.GetABValues(ctx, *user)
		require.Error(t, err)
		require.Nil(t, values)

		_, status, err = doRequestWithAuth(ctx, t, sat, user, http.MethodPost, "ab/hit/upgrade-account", nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)
	})
}
