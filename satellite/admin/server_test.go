// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
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
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()

		t.Run("NoAccess", func(t *testing.T) {
			response, err := http.Get("http://" + address.String())
			require.NoError(t, err)

			require.Equal(t, http.StatusForbidden, response.StatusCode)
			require.NoError(t, response.Body.Close())
		})

		t.Run("WrongAccess", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "http://"+address.String(), nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "wrong-key")

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusForbidden, response.StatusCode)
			require.NoError(t, response.Body.Close())
		})

		t.Run("WithAccess", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "http://"+address.String(), nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			// currently no main page so 404
			require.Equal(t, http.StatusNotFound, response.StatusCode)
			require.NoError(t, response.Body.Close())
		})
	})
}
