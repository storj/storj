// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"net/http"
	"testing"
	"time"

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

		tokenInfo, err := sat.API.Console.Service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
		require.NoError(t, err)

		client := http.Client{}

		req, err := http.NewRequestWithContext(ctx, "GET", "http://"+planet.Satellites[0].API.Console.Listener.Addr().String()+"/api/v0/ab/values", nil)
		require.NoError(t, err)

		expire := time.Now().AddDate(0, 0, 1)
		cookie := http.Cookie{
			Name:    "_tokenKey",
			Path:    "/",
			Value:   tokenInfo.Token.String(),
			Expires: expire,
		}

		req.AddCookie(&cookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		defer func() {
			err = resp.Body.Close()
			require.NoError(t, err)
		}()

		values, err := service.GetABValues(ctx, *user)
		require.Error(t, err)
		require.Nil(t, values)

		req, err = http.NewRequestWithContext(ctx, "POST", "http://"+planet.Satellites[0].API.Console.Listener.Addr().String()+"/api/v0/ab/hit/upgrade-account", nil)
		require.NoError(t, err)
		req.AddCookie(&cookie)

		hitResp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, hitResp.StatusCode)
		defer func() {
			err = hitResp.Body.Close()
			require.NoError(t, err)
		}()
	})
}
