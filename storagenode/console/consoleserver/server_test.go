// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleserver_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
)

func TestConsole(t *testing.T) {
	testplanet.Run(t,
		testplanet.Config{
			SatelliteCount:   1,
			StorageNodeCount: 1,
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			console := planet.StorageNodes[0].Console

			addr := console.Listener.Addr()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/api/sno", addr), nil)
			require.NoError(t, err)
			res, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			_ = res.Body.Close()
			require.Equal(t, http.StatusOK, res.StatusCode)

			req, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/api/sno/satellites", addr), nil)
			require.NoError(t, err)
			res, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			_ = res.Body.Close()
			require.Equal(t, http.StatusOK, res.StatusCode)

			req, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/api/sno/satellite/%s", addr, satellite.ID()), nil)
			require.NoError(t, err)
			res, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			_ = res.Body.Close()
			require.Equal(t, http.StatusOK, res.StatusCode)
		},
	)
}
