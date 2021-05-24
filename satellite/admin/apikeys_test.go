// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestAddApiKey(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		projectID := planet.Uplinks[0].Projects[0].ID

		keys, err := planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{Page: 1, Limit: 10})
		require.NoError(t, err)
		require.Len(t, keys.APIKeys, 1)

		body := strings.NewReader(`{"name":"Default"}`)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://"+address.String()+"/api/project/%s/apikey", projectID.String()), body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())

		var output struct {
			APIKey string `json:"apikey"`
		}

		err = json.Unmarshal(responseBody, &output)
		require.NoError(t, err)

		apikey, err := macaroon.ParseAPIKey(output.APIKey)
		require.NoError(t, err)
		require.NotNil(t, apikey)

		keys, err = planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{Page: 1, Limit: 10})
		require.NoError(t, err)
		require.Len(t, keys.APIKeys, 2)

		key, err := planet.Satellites[0].DB.Console().APIKeys().GetByHead(ctx, apikey.Head())
		require.NoError(t, err)
		require.Equal(t, "Default", key.Name)
	})
}

func TestDeleteApiKey(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		projectID := planet.Uplinks[0].Projects[0].ID

		keys, err := planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{Page: 1, Limit: 10})
		require.NoError(t, err)
		require.Len(t, keys.APIKeys, 1)

		apikey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()].Serialize()
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/apikey/%s", apikey), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Len(t, responseBody, 0)

		keys, err = planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{Page: 1, Limit: 10})
		require.NoError(t, err)
		require.Len(t, keys.APIKeys, 0)
	})
}

func TestDeleteApiKeyByName(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		projectID := planet.Uplinks[0].Projects[0].ID

		keys, err := planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{Page: 1, Limit: 10})
		require.NoError(t, err)
		require.Len(t, keys.APIKeys, 1)

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/project/%s/apikey/%s", projectID.String(), keys.APIKeys[0].Name), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Len(t, responseBody, 0)

		keys, err = planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{Page: 1, Limit: 10})
		require.NoError(t, err)
		require.Len(t, keys.APIKeys, 0)
	})
}
