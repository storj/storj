// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/private/httpmock"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console/valdi/valdiclient"
)

func TestValdiGetAPIKey(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.CloudGpusEnabled = true
				config.Valdi.SignRequests = false
				config.Valdi.SatelliteEmail = "storj@storj.test"
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		baseURL := "valdi/api-keys"

		user1, err := sat.DB.Console().Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].User[sat.ID()].Email, nil)
		require.NoError(t, err)
		p := planet.Uplinks[0].Projects[0]

		endpoint := fmt.Sprintf("%s/%s", baseURL, p.PublicID.String())

		t.Run("invalid project ID path param", func(t *testing.T) {
			body, status, err := doRequestWithAuth(ctx, t, sat, user1, http.MethodGet, baseURL+"/abc", nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, status)
			require.Contains(t, string(body), "invalid project id")
		})

		mockClient, transport := httpmock.NewClient()

		testValdiClient, err := valdiclient.New(zaptest.NewLogger(t), mockClient, sat.Config.Valdi.Config)
		require.NoError(t, err)

		*sat.API.Valdi.Client = *testValdiClient

		keySuccessResp := &valdiclient.CreateAPIKeyResponse{
			APIKey:            "1234",
			SecretAccessToken: "abc123",
		}

		jsonKey, err := json.Marshal(keySuccessResp)
		require.NoError(t, err)

		apiKeyEndpoint, err := url.JoinPath(sat.Config.Valdi.APIBaseURL, valdiclient.APIKeyPath)
		require.NoError(t, err)

		t.Run("success", func(t *testing.T) {
			transport.AddResponse(apiKeyEndpoint, httpmock.Response{
				StatusCode: http.StatusOK,
				Body:       string(jsonKey),
			})

			body, status, err := doRequestWithAuth(ctx, t, sat, user1, http.MethodGet, endpoint, nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)
			require.NotNil(t, body)
			apiKeyResp := &valdiclient.CreateAPIKeyResponse{}
			require.NoError(t, json.Unmarshal(body, apiKeyResp))
			require.Equal(t, keySuccessResp, apiKeyResp)
		})

		t.Run("errors", func(t *testing.T) {
			valdiErr := &valdiclient.ErrorMessage{
				Detail: "some valdi error",
			}

			errJson, err := json.Marshal(valdiErr)
			require.NoError(t, err)

			transport.AddResponse(apiKeyEndpoint, httpmock.Response{
				StatusCode: http.StatusConflict,
				Body:       string(errJson),
			})

			body, status, err := doRequestWithAuth(ctx, t, sat, user1, http.MethodGet, endpoint, nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusConflict, status)
			require.NotNil(t, body)

			require.Contains(t, string(body), valdiErr.Detail)
		})

	})
}
