// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity"
	"storj.io/storj/private/server"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/shared/debug"
	"storj.io/storj/shared/mudplanet"
	"storj.io/storj/shared/mudplanet/satellitetest"
	"storj.io/storj/shared/mudplanet/uplinktest"
	"storj.io/uplink"
)

// TestApiModules tests if API modules are correctly set up.
// This one tests only the wiring with starting minimal services.
func TestApiModules(t *testing.T) {
	mudplanet.Run(t, mudplanet.Config{
		Components: []mudplanet.Component{
			mudplanet.NewComponent("satellite",
				satellitetest.Satellite,
				mudplanet.WithModule(satellitetest.WithoutDB),
				mudplanet.WithRunning[debug.Wrapper]()),
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

func TestUploadInline(t *testing.T) {
	mudplanet.Run(t, satellitetest.WithDB(
		mudplanet.NewComponent("satellite", satellitetest.Satellite, mudplanet.WithRunning[*satellite.EndpointRegistration]()),
	),
		func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
			server := mudplanet.FindFirst[*server.Server](t, run, "satellite", 0)
			id := mudplanet.FindFirst[*identity.FullIdentity](t, run, "satellite", 0)

			uplinkCfg := uplink.Config{}
			access, err := satellitedb.GetTestApiKey(ctx, uplinkCfg, id.ID, server.Addr().String())
			require.NoError(t, err)

			uplink, err := uplinktest.NewUplink(access, uplinkCfg)
			require.NoError(t, err)
			err = uplink.Upload(ctx, "bucket1", "path/to/object", []byte("data"))
			require.NoError(t, err)
			data, err := uplink.Download(ctx, "bucket1", "path/to/object")
			require.NoError(t, err)
			require.Equal(t, "data", string(data))
			err = uplink.Delete(ctx, "bucket1", "path/to/object")
			require.NoError(t, err)
		})
}
