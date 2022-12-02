// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func Test_DeleteAPIKeyByNameAndProjectID(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		newUser := console.CreateUser{
			FullName:  "test_name",
			ShortName: "",
			Email:     "apikeytest@test.test",
		}

		user, err := sat.AddUser(ctx, newUser, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "apikeytest")
		require.NoError(t, err)

		secret, err := macaroon.NewSecret()
		require.NoError(t, err)

		key, err := macaroon.NewAPIKey(secret)
		require.NoError(t, err)

		apikey := console.APIKeyInfo{
			Name:      "test",
			ProjectID: project.ID,
			Secret:    secret,
		}

		created, err := sat.DB.Console().APIKeys().Create(ctx, key.Head(), apikey)
		require.NoError(t, err)

		// we are using full name as a password
		tokenInfo, err := sat.API.Console.Service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
		require.NoError(t, err)

		client := http.Client{}

		req, err := http.NewRequestWithContext(ctx, "DELETE", "http://"+planet.Satellites[0].API.Console.Listener.Addr().String()+"/api/v0/api-keys/delete-by-name?name="+apikey.Name+"&projectID="+project.ID.String(), nil)
		require.NoError(t, err)

		expire := time.Now().AddDate(0, 0, 1)
		cookie := http.Cookie{
			Name:    "_tokenKey",
			Path:    "/",
			Value:   tokenInfo.Token.String(),
			Expires: expire,
		}

		req.AddCookie(&cookie)

		result, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, result.StatusCode)

		keyAfterDelete, err := sat.DB.Console().APIKeys().Get(ctx, created.ID)
		require.Error(t, err)
		require.Nil(t, keyAfterDelete)

		defer func() {
			err = result.Body.Close()
			require.NoError(t, err)
		}()
	})
}
