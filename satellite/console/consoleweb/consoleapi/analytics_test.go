// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
)

func TestAccountObjectCreated(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Analytics.Enabled = true
				config.Analytics.HubSpot.ClientSecret = "supersecret"
				config.Analytics.HubSpot.AccountObjectCreatedWebhookEnabled = true
				config.Analytics.HubSpot.WebhookRequestLifetime = time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		hubspotConfig := sat.Config.Analytics.HubSpot

		sat.API.Analytics.Service.TestSetSatelliteExternalAddress("http://" + sat.API.Console.Listener.Addr().String())

		newUser := console.CreateUser{
			FullName: "Hubspot Test",
			Email:    "hubspot@example.test",
		}

		user, err := sat.AddUser(ctx, newUser, 1)
		require.NoError(t, err)

		requestBody := analytics.AccountObjectCreatedRequest{
			UserID:   user.ID.String(),
			ObjectID: "1234567890",
		}

		jsonBody, err := json.Marshal(requestBody)
		require.NoError(t, err)

		link, err := url.JoinPath(sat.ConsoleURL(), hubspotConfig.AccountObjectCreatedWebhookEndpoint)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, link, bytes.NewBuffer(jsonBody))
		require.NoError(t, err)

		timestampHeader := strconv.FormatInt(time.Now().UnixMilli(), 10)
		req.Header.Set("x-hubspot-request-timestamp", timestampHeader)

		rawString := http.MethodPost + link + string(jsonBody) + timestampHeader
		h := hmac.New(sha256.New, []byte(hubspotConfig.ClientSecret))
		_, err = h.Write([]byte(rawString))
		require.NoError(t, err)

		hashedString := base64.StdEncoding.EncodeToString(h.Sum(nil))

		req.Header.Set("x-hubspot-signature-v3", hashedString)
		req.Header.Set("Content-Type", "application/json")

		result, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, result.StatusCode)

		defer func() {
			err = result.Body.Close()
			require.NoError(t, err)
		}()

		// Check that the user's hubspot object ID was updated.
		user, err = sat.DB.Console().Users().Get(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, user.HubspotObjectID)
		require.Equal(t, requestBody.ObjectID.String(), *user.HubspotObjectID)
	})
}
