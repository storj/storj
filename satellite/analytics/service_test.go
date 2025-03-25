// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package analytics_test

import (
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
)

func TestValidateAccountObjectCreatedRequestSignature(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Analytics.Enabled = true
				config.Analytics.HubSpot.ClientSecret = "supersecret"
				config.Analytics.HubSpot.WebhookRequestLifetime = time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		hubspotConfig := sat.Config.Analytics.HubSpot
		analyticsService := sat.API.Analytics.Service

		request := analytics.AccountObjectCreatedRequest{
			UserID:   "test-user-id",
			ObjectID: "1234567890",
		}

		now := time.Now()
		nowMilliStr := strconv.FormatInt(now.UnixMilli(), 10)
		wrongTimestamp := now.Add(2 * time.Hour)
		wrongMilliStr := strconv.FormatInt(wrongTimestamp.UnixMilli(), 10)

		// Wrong timestamp.
		err := analyticsService.ValidateAccountObjectCreatedRequestSignature(request, "", wrongMilliStr)
		require.Error(t, err)

		// Wrong signature.
		err = analyticsService.ValidateAccountObjectCreatedRequestSignature(request, "", nowMilliStr)
		require.Error(t, err)

		jsonBytes, err := json.Marshal(request)
		require.NoError(t, err)

		link, err := url.JoinPath(sat.Config.Console.ExternalAddress, hubspotConfig.AccountObjectCreatedWebhookEndpoint)
		require.NoError(t, err)

		expectedRawString := http.MethodPost + link + string(jsonBytes) + nowMilliStr

		h := hmac.New(sha256.New, []byte(hubspotConfig.ClientSecret))
		_, err = h.Write([]byte(expectedRawString))
		require.NoError(t, err)

		expectedHashedString := base64.StdEncoding.EncodeToString(h.Sum(nil))

		// Correct request.
		err = analyticsService.ValidateAccountObjectCreatedRequestSignature(request, expectedHashedString, nowMilliStr)
		require.NoError(t, err)
	})
}
