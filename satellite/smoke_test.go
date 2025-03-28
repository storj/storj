// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
	trustmud "storj.io/storj/satellite/trust/mud"
	"storj.io/storj/shared/debug"
	"storj.io/storj/shared/mud"
	"storj.io/storj/shared/mudplanet"
)

// TestApiModules tests if API modules are correctly set up.
// This one tests only the wiring with starting minimal services.
func TestApiModules(t *testing.T) {
	mudplanet.Run(t, mudplanet.Config{
		Components: []mudplanet.Component{
			{
				Name: "satellite",
				Modules: mudplanet.Modules{
					trustmud.Module,
					satellitedb.Module,
					satellite.Module,
				},
				Selector: mud.Or(
					mud.SelectIfExists[debug.Wrapper](),
				),
			},
		},
	}, func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		wrapper := mudplanet.FindFirst[debug.Wrapper](t, run, "satellite", 0)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+wrapper.Listener.Addr().String()+"/debug/vars", nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() {
			_ = resp.Body.Close()
		}()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
