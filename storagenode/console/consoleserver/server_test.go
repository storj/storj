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

			t.Run("test endpoints", func(t *testing.T) {
				addr := console.Listener.Addr()

				req, err := http.Get(fmt.Sprintf("http://%s/api/dashboard", addr))
				require.NoError(t, err)
				require.NotNil(t, req)
				_ = req.Body.Close()
				require.Equal(t, http.StatusOK, req.StatusCode)

				req, err = http.Get(fmt.Sprintf("http://%s/api/satellites", addr))
				require.NoError(t, err)
				require.NotNil(t, req)
				_ = req.Body.Close()
				require.Equal(t, http.StatusOK, req.StatusCode)

				req, err = http.Get(fmt.Sprintf("http://%s/api/satellite/%s", addr, satellite.ID()))
				require.NoError(t, err)
				require.NotNil(t, req)
				_ = req.Body.Close()
				require.Equal(t, http.StatusOK, req.StatusCode)
			})
		},
	)
}
