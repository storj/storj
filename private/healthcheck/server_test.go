// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package healthcheck_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/healthcheck"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestHealthCheck(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.HealthCheck.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		server := planet.Satellites[0].API.HealthCheck.Server
		client := http.Client{}

		root := "http://" + server.TestGetAddress() + "/health"

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, root, nil)
		require.NoError(t, err)

		resp, err := client.Do(request)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())

		check1 := dummyHealthCheck{
			name:    "check1",
			healthy: true,
		}
		err = server.AddCheck(check1)
		require.NoError(t, err)

		err = server.AddCheck(check1)
		require.Error(t, err)
		require.ErrorIs(t, err, healthcheck.ErrCheckExists)

		check2 := dummyHealthCheck{
			name:    "check2",
			healthy: true,
		}
		err = server.AddCheck(check2)
		require.NoError(t, err)

		var checkResponse map[string]bool

		resp, err = client.Do(request)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&checkResponse))
		require.True(t, checkResponse[check1.name])
		require.True(t, checkResponse[check2.name])
		require.NoError(t, resp.Body.Close())

		check3 := dummyHealthCheck{
			name:    "check3",
			healthy: false,
		}
		err = server.AddCheck(check3)
		require.NoError(t, err)

		resp, err = client.Do(request)
		require.NoError(t, err)
		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&checkResponse))
		require.True(t, checkResponse[check1.name])
		require.True(t, checkResponse[check2.name])
		require.False(t, checkResponse[check3.name])
		require.NoError(t, resp.Body.Close())

		for _, check := range []dummyHealthCheck{check1, check2, check3} {
			request, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", root, check.name), nil)
			require.NoError(t, err)

			expectedStatus := http.StatusOK
			if !check.healthy {
				expectedStatus = http.StatusServiceUnavailable
			}

			var body struct {
				Healthy bool `json:"healthy"`
			}
			resp, err = client.Do(request)
			require.NoError(t, err)
			require.Equal(t, expectedStatus, resp.StatusCode)
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			require.Equal(t, check.healthy, body.Healthy)
			require.NoError(t, resp.Body.Close())
		}

		request, err = http.NewRequestWithContext(ctx, http.MethodGet, root+"/fake-check", nil)
		require.NoError(t, err)

		resp, err = client.Do(request)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})
}

type dummyHealthCheck struct {
	name    string
	healthy bool
}

func (d dummyHealthCheck) Healthy(_ context.Context) bool {
	return d.healthy
}

func (d dummyHealthCheck) Name() string {
	return d.name
}
