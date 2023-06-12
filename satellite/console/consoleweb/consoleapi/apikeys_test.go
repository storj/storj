// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"encoding/json"
	"io"
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

		// we are using full name as a password
		tokenInfo, err := sat.API.Console.Service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
		require.NoError(t, err)

		client := http.Client{}

		expire := time.Now().AddDate(0, 0, 1)
		cookie := http.Cookie{
			Name:    "_tokenKey",
			Path:    "/",
			Value:   tokenInfo.Token.String(),
			Expires: expire,
		}

		deleteTestFunc := func(request *http.Request) func(t *testing.T) {
			return func(t *testing.T) {
				created, err := sat.DB.Console().APIKeys().Create(ctx, key.Head(), apikey)
				require.NoError(t, err)

				request.AddCookie(&cookie)

				result, err := client.Do(request)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, result.StatusCode)

				keyAfterDelete, err := sat.DB.Console().APIKeys().Get(ctx, created.ID)
				require.Error(t, err)
				require.Nil(t, keyAfterDelete)

				defer func() {
					err = result.Body.Close()
					require.NoError(t, err)
				}()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "http://"+planet.Satellites[0].API.Console.Listener.Addr().String()+"/api/v0/api-keys/delete-by-name?name="+apikey.Name+"&projectID="+project.ID.String(), nil)
		require.NoError(t, err)
		t.Run("delete by name and projectID", deleteTestFunc(req))

		req, err = http.NewRequestWithContext(ctx, http.MethodDelete, "http://"+planet.Satellites[0].API.Console.Listener.Addr().String()+"/api/v0/api-keys/delete-by-name?name="+apikey.Name+"&publicID="+project.PublicID.String(), nil)
		require.NoError(t, err)
		t.Run("delete by name and publicID", deleteTestFunc(req))
	})
}

func Test_GetAllAPIKeyNamesByProjectID(t *testing.T) {
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
			Email:     "apikeytest1@test.test",
		}

		user, err := sat.AddUser(ctx, newUser, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "apikeytest")
		require.NoError(t, err)

		// we are using full name as a password
		tokenInfo, err := sat.API.Console.Service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
		require.NoError(t, err)

		client := http.Client{}

		expire := time.Now().AddDate(0, 0, 1)
		cookie := http.Cookie{
			Name:    "_tokenKey",
			Path:    "/",
			Value:   tokenInfo.Token.String(),
			Expires: expire,
		}

		secret, err := macaroon.NewSecret()
		require.NoError(t, err)

		key, err := macaroon.NewAPIKey(secret)
		require.NoError(t, err)

		apikey := console.APIKeyInfo{
			Name:      "test",
			ProjectID: project.ID,
			Secret:    secret,
		}

		secret1, err := macaroon.NewSecret()
		require.NoError(t, err)

		key1, err := macaroon.NewAPIKey(secret1)
		require.NoError(t, err)

		apikey1 := console.APIKeyInfo{
			Name:      "test1",
			ProjectID: project.ID,
			Secret:    secret1,
		}

		created, err := sat.DB.Console().APIKeys().Create(ctx, key.Head(), apikey)
		require.NoError(t, err)

		created1, err := sat.DB.Console().APIKeys().Create(ctx, key1.Head(), apikey1)
		require.NoError(t, err)

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+planet.Satellites[0].API.Console.Listener.Addr().String()+"/api/v0/api-keys/api-key-names?projectID="+project.ID.String(), nil)
		require.NoError(t, err)

		request.AddCookie(&cookie)

		result, err := client.Do(request)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, result.StatusCode)

		body, err := io.ReadAll(result.Body)
		require.NoError(t, err)

		var output []string

		err = json.Unmarshal(body, &output)
		require.NoError(t, err)

		require.Equal(t, 2, len(output))
		require.Equal(t, created.Name, output[0])
		require.Equal(t, created1.Name, output[1])

		defer func() {
			err = result.Body.Close()
			require.NoError(t, err)
		}()
	})
}
