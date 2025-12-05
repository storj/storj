// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestDeleteAPIKeyByNameAndProjectID(t *testing.T) {
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

		deleteTestFunc := func(endpointSuffix string) func(t *testing.T) {
			return func(t *testing.T) {
				created, err := sat.DB.Console().APIKeys().Create(ctx, key.Head(), apikey)
				require.NoError(t, err)

				endpoint := "api-keys/delete-by-name?name=" + apikey.Name
				_, status, err := doRequestWithAuth(ctx, t, sat, user, http.MethodDelete, endpoint+endpointSuffix, nil)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, status)

				keyAfterDelete, err := sat.DB.Console().APIKeys().Get(ctx, created.ID)
				require.Error(t, err)
				require.Nil(t, keyAfterDelete)
			}
		}

		t.Run("delete by name and projectID", deleteTestFunc("&projectID="+project.ID.String()))
		t.Run("delete by name and publicID", deleteTestFunc("&publicID="+project.PublicID.String()))
	})
}

func TestGetAllAPIKeyNamesByProjectID(t *testing.T) {
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

		endpoint := "api-keys/api-key-names?projectID=" + project.ID.String()
		body, status, err := doRequestWithAuth(ctx, t, sat, user, http.MethodGet, endpoint, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		var output []string

		err = json.Unmarshal(body, &output)
		require.NoError(t, err)

		require.Equal(t, 2, len(output))
		require.Equal(t, created.Name, output[0])
		require.Equal(t, created1.Name, output[1])
	})
}

func TestCreateAuditableAPIKey(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		keysDB := sat.DB.Console().APIKeys()
		service := sat.API.Console.Service

		newUser := console.CreateUser{
			FullName: "test_name",
			Email:    "apikeytest@example.test",
		}

		user, err := sat.AddUser(ctx, newUser, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "test")
		require.NoError(t, err)

		endpoint := "api-keys/create/" + project.PublicID.String()
		buf := bytes.NewBufferString("testName")

		_, status, err := doRequestWithAuth(ctx, t, sat, user, http.MethodPost, endpoint, buf)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		info, err := keysDB.GetByNameAndProjectID(ctx, "testName", project.ID)
		require.NoError(t, err)
		require.NotNil(t, info)
		// object lock is enabled by default.
		require.Equal(t, macaroon.APIKeyVersionObjectLock, info.Version)
		require.False(t, info.Version.SupportsAuditability())

		service.TestSetAuditableAPIKeyProjects(map[string]struct{}{project.PublicID.String(): {}})
		buf = bytes.NewBufferString("testName1")

		_, status, err = doRequestWithAuth(ctx, t, sat, user, http.MethodPost, endpoint, buf)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		info, err = keysDB.GetByNameAndProjectID(ctx, "testName1", project.ID)
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Equal(t, macaroon.APIKeyVersionObjectLock|macaroon.APIKeyVersionAuditable, info.Version)
		require.True(t, info.Version.SupportsAuditability())
	})
}
