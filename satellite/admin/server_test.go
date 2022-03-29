// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestBasic(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
				config.Admin.StaticDir = "ui/build"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()
		baseURL := "http://" + address.String()

		t.Run("UI", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/.keep", nil)
			require.NoError(t, err)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, response.StatusCode)

			content, err := ioutil.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.Empty(t, content)
			require.NoError(t, err)
		})

		t.Run("NoAccess", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/projects/some-id", nil)
			require.NoError(t, err)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusForbidden, response.StatusCode)
			require.Equal(t, "application/json", response.Header.Get("Content-Type"))

			body, err := ioutil.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.NoError(t, err)
			require.Equal(t, `{"error":"Forbidden","detail":""}`, string(body))
		})

		t.Run("WrongAccess", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/users/alice@storj.test", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "wrong-key")

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusForbidden, response.StatusCode)
			require.Equal(t, "application/json", response.Header.Get("Content-Type"))

			body, err := ioutil.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.NoError(t, err)
			require.Equal(t, `{"error":"Forbidden","detail":""}`, string(body))
		})

		t.Run("WithAccess", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			// currently no main page so 404
			require.Equal(t, http.StatusNotFound, response.StatusCode)
			require.Equal(t, "text/plain; charset=utf-8", response.Header.Get("Content-Type"))

			body, err := ioutil.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.NoError(t, err)
			require.Contains(t, string(body), "not found")
		})
	})
}
