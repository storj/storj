// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleserver_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestConsole(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 1, 0)
	require.NoError(t, err)

	planet.Start(ctx)

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

	require.NoError(t, planet.Shutdown())
}
