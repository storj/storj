// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/console"
	"storj.io/storj/storagenode/reputation"
)

func TestService_GetDashboardData(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		// to populate SN reputation DB
		err := planet.StorageNodes[0].Reputation.Chore.RunOnce(ctx)
		require.NoError(t, err)

		{
			dashboard, err := planet.StorageNodes[0].Console.Service.GetDashboardData(ctx)
			require.NoError(t, err)

			require.Equal(t, dashboard.NodeID, planet.StorageNodes[0].ID())
			require.Equal(t, 2, len(dashboard.Satellites))

			// Initially VettedAt should be nil (not yet vetted)
			for _, sat := range dashboard.Satellites {
				require.Nil(t, sat.VettedAt)
			}
		}
		{ // test VettedAt field with vetted satellite
			vettedTime := time.Now().UTC()
			stats := reputation.Stats{
				SatelliteID: planet.Satellites[0].ID(),
				VettedAt:    &vettedTime,
			}
			err := planet.StorageNodes[0].DB.Reputation().Store(ctx, stats)
			require.NoError(t, err)

			dashboard, err := planet.StorageNodes[0].Console.Service.GetDashboardData(ctx)
			require.NoError(t, err)

			// Find the vetted satellite in dashboard
			var vettedSat *console.SatelliteInfo
			for i := range dashboard.Satellites {
				if dashboard.Satellites[i].ID == planet.Satellites[0].ID() {
					vettedSat = &dashboard.Satellites[i]
					break
				}
			}
			require.NotNil(t, vettedSat)
			require.NotNil(t, vettedSat.VettedAt)
			require.Equal(t, vettedTime, *vettedSat.VettedAt)

			// Test GetSatelliteData includes VettedAt
			satelliteData, err := planet.StorageNodes[0].Console.Service.GetSatelliteData(ctx, planet.Satellites[0].ID())
			require.NoError(t, err)
			require.NotNil(t, satelliteData.VettedAt)
			require.Equal(t, vettedTime, *satelliteData.VettedAt)
		}
		{ // add untrusted satellite
			stats := reputation.Stats{
				SatelliteID: testrand.NodeID(),
			}
			err := planet.StorageNodes[0].DB.Reputation().Store(ctx, stats)
			require.NoError(t, err)

			// GetDashboardData shouldn't error if one of SN satellites is untrusted
			dashboard, err := planet.StorageNodes[0].Console.Service.GetDashboardData(ctx)
			require.NoError(t, err)

			require.Equal(t, dashboard.NodeID, planet.StorageNodes[0].ID())
			require.Equal(t, 2, len(dashboard.Satellites))
		}
	})
}

func TestService_GetAllSatellitesData(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		{
			_, err := planet.StorageNodes[0].Console.Service.GetAllSatellitesData(ctx)
			require.NoError(t, err)
		}
		// TODO figure out how add untrusted satellite to storagenode/trust/service and test GetAllSatellitesData
	})
}
