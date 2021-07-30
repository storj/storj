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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
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

func TestListAPIKeys(t *testing.T) {
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
		var (
			sat       = planet.Satellites[0]
			authToken = planet.Satellites[0].Config.Console.AuthToken
			address   = sat.Admin.Admin.Listener.Addr()
		)

		project, err := sat.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)

		linkAPIKey := "http://" + address.String() + "/api/project/" + project.ID.String() + "/apikey"
		linkAPIKeys := "http://" + address.String() + "/api/project/" + project.ID.String() + "/apikeys"

		{ // Delete initial API Keys to run this test

			page, err := sat.DB.Console().APIKeys().GetPagedByProjectID(
				ctx, project.ID, console.APIKeyCursor{
					Limit: 50, Page: 1, Order: console.KeyName, OrderDirection: console.Ascending,
				},
			)
			require.NoError(t, err)

			// Ensure that we are getting all the initial keys with one single page.
			require.Len(t, page.APIKeys, int(page.TotalCount))

			for _, ak := range page.APIKeys {
				require.NoError(t, sat.DB.Console().APIKeys().Delete(ctx, ak.ID))
			}
		}

		// Check get initial list of API keys.
		assertGet(ctx, t, linkAPIKeys, "[]", authToken)

		{ // Create 2 new API Key.
			body := assertReq(ctx, t, linkAPIKey, http.MethodPost, `{"name": "first"}`, http.StatusOK, "", authToken)
			apiKey := struct {
				Apikey string `json:"apikey"`
			}{}
			require.NoError(t, json.Unmarshal(body, &apiKey))
			require.NotEmpty(t, apiKey.Apikey)

			body = assertReq(ctx, t, linkAPIKey, http.MethodPost, `{"name": "second"}`, http.StatusOK, "", authToken)
			require.NoError(t, json.Unmarshal(body, &apiKey))
			require.NotEmpty(t, apiKey.Apikey)

			// TODO: figure out how to create an API Key associated to a partner.
			// sat.DB.Console().APIKeys.Update only allows to update the API Key name
		}

		// Check get list of API keys.
		body := assertReq(ctx, t, linkAPIKeys, http.MethodGet, "", http.StatusOK, "", authToken)

		var apiKeys []struct {
			ID        string `json:"id"`
			ProjectID string `json:"projectId"`
			Name      string `json:"name"`
			PartnerID string `json:"partnerID"`
			CreatedAt string `json:"createdAt"`
		}
		require.NoError(t, json.Unmarshal(body, &apiKeys))
		require.Len(t, apiKeys, 2)
		{ // Assert API keys info.
			a := apiKeys[0]
			assert.NotEmpty(t, a.ID, "API key ID")
			assert.Equal(t, "first", a.Name, "API key name")
			assert.Equal(t, project.ID.String(), a.ProjectID, "API key project ID")
			assert.Equal(t, uuid.UUID{}.String(), a.PartnerID, "API key partner ID")
			assert.NotEmpty(t, a.CreatedAt, "API key created at")

			a = apiKeys[1]
			assert.NotEmpty(t, a.ID, "API key ID")
			assert.Equal(t, "second", a.Name, "API key name")
			assert.Equal(t, project.ID.String(), a.ProjectID, "API key project ID")
			assert.Equal(t, uuid.UUID{}.String(), a.PartnerID, "API key partner ID")
			assert.NotEmpty(t, a.CreatedAt, "API key created at")
		}

		{ // Delete one API key to check that the endpoint just returns one.
			id, err := uuid.FromString(apiKeys[1].ID)
			require.NoError(t, err)
			require.NoError(t, sat.DB.Console().APIKeys().Delete(ctx, id))
		}

		body = assertReq(ctx, t, linkAPIKeys, http.MethodGet, "", http.StatusOK, "", authToken)
		require.NoError(t, json.Unmarshal(body, &apiKeys))
		require.Len(t, apiKeys, 1)
		{ // Assert API keys info.
			a := apiKeys[0]
			assert.NotEmpty(t, a.ID, "API key ID")
			assert.Equal(t, "first", a.Name, "API key name")
			assert.Equal(t, project.ID.String(), a.ProjectID, "API key project ID")
			assert.Equal(t, uuid.UUID{}.String(), a.PartnerID, "API key partner ID")
			assert.NotEmpty(t, a.CreatedAt, "API key created at")
		}

		{ // Delete the one API key that last to check that the endpoint just returns none.
			id, err := uuid.FromString(apiKeys[0].ID)
			require.NoError(t, err)
			require.NoError(t, sat.DB.Console().APIKeys().Delete(ctx, id))
		}

		// Check get initial list of API keys.
		assertGet(ctx, t, linkAPIKeys, "[]", authToken)
	})
}
