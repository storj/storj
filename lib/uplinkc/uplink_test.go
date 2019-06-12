// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"storj.io/storj/internal/testplanet"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
)

func TestUplink(t *testing.T) {
	var cerr Cpchar

	var config CUplinkConfig
	config.Volatile.TLS.SkipPeerCAWhitelist = 1

	uplink := new_uplink(config, &cerr)
	require.Nil(t, cerr)
	require.NotEmpty(t, uplink)

	close_uplink(uplink, &cerr)
	require.Nil(t, cerr)
}

func TestProject(t *testing.T) {
	RunPlanet(t, func(ctx *testcontext.Context, planet *testplanet.Planet) {
		satelliteAddr := planet.Satellites[0].Addr()
		apikeyStr := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		{
			var config CUplinkConfig
			config.Volatile.TLS.SkipPeerCAWhitelist = 1

			var cerr Cpchar
			uplink := new_uplink(config, &cerr)
			require.Nil(t, cerr)
			require.NotEmpty(t, uplink)

			defer func() {
				close_uplink(uplink, &cerr)
				require.Nil(t, cerr)
			}()

			{
				capikeyStr := CString(apikeyStr)
				defer CFree(unsafe.Pointer(capikeyStr))

				apikey := parse_api_key(capikeyStr, &cerr)
				require.Nil(t, cerr)
				require.NotEmpty(t, apikey)
				defer free_api_key(apikey)

				cSatelliteAddr := CString(satelliteAddr)
				defer CFree(unsafe.Pointer(cSatelliteAddr))

				project := open_project(uplink, cSatelliteAddr, apikey, &cerr)
				require.Nil(t, cerr)
				require.NotEmpty(t, uplink)

				defer func() {
					close_project(project, &cerr)
					require.Nil(t, cerr)
				}()
			}
		}
	})
}
