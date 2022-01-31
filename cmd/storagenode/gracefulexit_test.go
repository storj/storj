// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package main

import (
	"os"
	"testing"
	"text/tabwriter"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestGracefulExitTooEarly(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				if index == 1 {
					config.GracefulExit.NodeMinAgeInMonths = 0
				} else {
					config.GracefulExit.NodeMinAgeInMonths = 6
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.StorageNodes[0].Server.PrivateAddr().String()

		client, err := dialGracefulExitClient(ctx, address)
		require.NoError(t, err)

		response, err := client.gracefulExitFeasibility(ctx, planet.Satellites[0].ID())
		require.NoError(t, err)
		require.Equal(t, response.IsAllowed, false)

		response2, err := client.gracefulExitFeasibility(ctx, planet.Satellites[1].ID())
		require.NoError(t, err)
		require.Equal(t, response2.IsAllowed, true)
	})
}

func TestGracefulExitInit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				if index == 1 {
					config.GracefulExit.NodeMinAgeInMonths = 0
				} else {
					config.GracefulExit.NodeMinAgeInMonths = 6
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var satelliteIDs []storj.NodeID
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		address := planet.StorageNodes[0].Server.PrivateAddr().String()
		satelliteIDs = append(satelliteIDs, planet.Satellites[0].ID(), planet.Satellites[1].ID())

		client, err := dialGracefulExitClient(ctx, address)
		require.NoError(t, err)

		err = gracefulExitInit(ctx, satelliteIDs, w, client)
		require.Error(t, err)
	})
}
