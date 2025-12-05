// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestRESTKeys(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, runTests)
}

func TestRESTKeys_WithNewTable(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
				config.Console.UseNewRestKeysTable = true
			},
		},
	}, runTests)
}

func runTests(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
	address := planet.Satellites[0].Admin.Admin.Listener.Addr()
	sat := planet.Satellites[0]
	keyService := sat.API.Console.RestKeys

	user, err := planet.Satellites[0].DB.Console().Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].Projects[0].Owner.Email, nil)
	require.NoError(t, err)

	t.Run("create with default expiration", func(t *testing.T) {
		body := strings.NewReader(`{"expiration":""}`)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://"+address.String()+"/api/restkeys/%s", user.Email), body)
		require.NoError(t, err)
		req.Header.Set("Authorization", sat.Config.Console.AuthToken)

		// get current time to check against ExpiresAt
		now := time.Now()

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))

		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())

		var output struct {
			APIKey    string    `json:"apikey"`
			ExpiresAt time.Time `json:"expiresAt"`
		}

		err = json.Unmarshal(responseBody, &output)
		require.NoError(t, err)

		userID, exp, err := keyService.GetUserAndExpirationFromKey(ctx, output.APIKey)
		require.NoError(t, err)
		require.Equal(t, user.ID, userID)
		require.False(t, exp.IsZero())
		require.False(t, exp.Before(now))

		// check the expiration is around the time we expect
		defaultExpiration := sat.Config.Console.RestAPIKeys.DefaultExpiration
		require.True(t, output.ExpiresAt.After(now.Add(defaultExpiration)))
		require.True(t, output.ExpiresAt.Before(now.Add(defaultExpiration+time.Hour)))
	})

	t.Run("create with custom expiration", func(t *testing.T) {
		durationString := "3h"
		body := strings.NewReader(fmt.Sprintf(`{"expiration":"%s"}`, durationString))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://"+address.String()+"/api/restkeys/%s", user.Email), body)
		require.NoError(t, err)
		req.Header.Set("Authorization", sat.Config.Console.AuthToken)

		// get current time to check against ExpiresAt
		now := time.Now()

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))

		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())

		var output struct {
			APIKey    string    `json:"apikey"`
			ExpiresAt time.Time `json:"expiresAt"`
		}

		err = json.Unmarshal(responseBody, &output)
		require.NoError(t, err)

		userID, exp, err := keyService.GetUserAndExpirationFromKey(ctx, output.APIKey)
		require.NoError(t, err)
		require.Equal(t, user.ID, userID)
		require.False(t, exp.IsZero())
		require.False(t, exp.Before(now))

		// check the expiration is around the time we expect
		durationTime, err := time.ParseDuration(durationString)
		require.NoError(t, err)
		require.True(t, output.ExpiresAt.After(now.Add(durationTime)))
		require.True(t, output.ExpiresAt.Before(now.Add(durationTime+time.Hour)))
	})

	t.Run("revoke key", func(t *testing.T) {
		dur := time.Hour
		apiKey, expiresAt, err := keyService.CreateNoAuth(ctx, user.ID, &dur)
		require.NoError(t, err)
		require.False(t, expiresAt.IsZero())

		req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("http://"+address.String()+"/api/restkeys/%s/revoke", apiKey), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", sat.Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.NoError(t, response.Body.Close())

		_, _, err = keyService.GetUserAndExpirationFromKey(ctx, apiKey)
		require.Error(t, err)
	})
}
