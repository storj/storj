// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package forgetsatellite_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/internalpb"
	"storj.io/storj/storagenode/satellites"
)

func TestEndpoint_InitForgetSatellite(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 3, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		storagenode := planet.StorageNodes[0]

		// pause the chore
		storagenode.ForgetSatellite.Chore.Loop.Pause()

		t.Run("new request", func(t *testing.T) {
			cleanupSatellite := planet.Satellites[0]

			// mark satellite as untrusted
			err := storagenode.DB.Satellites().UpdateSatelliteStatus(ctx, cleanupSatellite.ID(), satellites.Untrusted)
			require.NoError(t, err)

			req := &internalpb.InitForgetSatelliteRequest{
				SatelliteId: cleanupSatellite.ID(),
			}

			resp, err := storagenode.ForgetSatellite.Endpoint.InitForgetSatellite(ctx, req)
			require.NoError(t, err)
			require.Equal(t, true, resp.InProgress)
			require.Equal(t, cleanupSatellite.ID(), resp.SatelliteId)

			satellite, err := storagenode.DB.Satellites().GetSatellite(ctx, cleanupSatellite.ID())
			require.NoError(t, err)
			require.Equal(t, cleanupSatellite.ID(), satellite.SatelliteID)
			require.Equal(t, satellites.CleanupInProgress, satellite.Status)
		})

		t.Run("satellite is not untrusted", func(t *testing.T) {
			cleanupSatellite := planet.Satellites[1]

			req := &internalpb.InitForgetSatelliteRequest{
				SatelliteId: cleanupSatellite.ID(),
			}

			resp, err := storagenode.ForgetSatellite.Endpoint.InitForgetSatellite(ctx, req)
			require.Error(t, err, "satellite is not untrusted")
			require.Nil(t, resp)

			satellite, err := storagenode.DB.Satellites().GetSatellite(ctx, cleanupSatellite.ID())
			require.NoError(t, err)
			require.Equal(t, cleanupSatellite.ID(), satellite.SatelliteID)
			require.Equal(t, satellites.Normal, satellite.Status)
		})

		t.Run("satellite is not untrusted but force cleanup", func(t *testing.T) {
			cleanupSatellite := planet.Satellites[2]

			req := &internalpb.InitForgetSatelliteRequest{
				SatelliteId:  cleanupSatellite.ID(),
				ForceCleanup: true,
			}

			resp, err := storagenode.ForgetSatellite.Endpoint.InitForgetSatellite(ctx, req)
			require.NoError(t, err)
			require.Equal(t, true, resp.InProgress)
			require.Equal(t, cleanupSatellite.ID(), resp.SatelliteId)

			satellite, err := storagenode.DB.Satellites().GetSatellite(ctx, cleanupSatellite.ID())
			require.NoError(t, err)
			require.Equal(t, cleanupSatellite.ID(), satellite.SatelliteID)
			require.Equal(t, satellites.CleanupInProgress, satellite.Status)
		})
	})
}

func TestEndpoint_GetUntrustedSatellites(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 3, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		storagenode := planet.StorageNodes[0]
		satellite := planet.Satellites[1]

		resp, err := storagenode.ForgetSatellite.Endpoint.GetUntrustedSatellites(ctx, &internalpb.GetUntrustedSatellitesRequest{})
		require.NoError(t, err)
		require.Len(t, resp.SatelliteIds, 0)

		// mark satellite 1 as untrusted
		err = storagenode.DB.Satellites().UpdateSatelliteStatus(ctx, satellite.ID(), satellites.Untrusted)
		require.NoError(t, err)

		resp, err = storagenode.ForgetSatellite.Endpoint.GetUntrustedSatellites(ctx, &internalpb.GetUntrustedSatellitesRequest{})
		require.NoError(t, err)
		require.Len(t, resp.SatelliteIds, 1)
		require.Equal(t, satellite.ID(), resp.SatelliteIds[0])
	})
}

func TestEndpoint_ForgetSatelliteStatus(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		storagenode := planet.StorageNodes[0]
		cleanupSatellite := planet.Satellites[0]

		// pause the chore
		storagenode.ForgetSatellite.Chore.Loop.Pause()

		// mark satellite as untrusted
		err := storagenode.DB.Satellites().UpdateSatelliteStatus(ctx, cleanupSatellite.ID(), satellites.Untrusted)
		require.NoError(t, err)

		// send forget satellite request
		resp, err := storagenode.ForgetSatellite.Endpoint.InitForgetSatellite(ctx, &internalpb.InitForgetSatelliteRequest{
			SatelliteId: cleanupSatellite.ID(),
		})
		require.NoError(t, err)
		require.Equal(t, true, resp.InProgress)
		require.Equal(t, cleanupSatellite.ID(), resp.SatelliteId)

		// check status
		status, err := storagenode.ForgetSatellite.Endpoint.ForgetSatelliteStatus(ctx, &internalpb.ForgetSatelliteStatusRequest{
			SatelliteId: cleanupSatellite.ID(),
		})
		require.NoError(t, err)
		require.Equal(t, true, status.InProgress)
		require.Equal(t, cleanupSatellite.ID(), status.SatelliteId)

		// trigger the chore
		storagenode.ForgetSatellite.Chore.Loop.TriggerWait()
		// check that the cleanup was successful
		status, err = storagenode.ForgetSatellite.Endpoint.ForgetSatelliteStatus(ctx, &internalpb.ForgetSatelliteStatusRequest{
			SatelliteId: cleanupSatellite.ID(),
		})
		require.NoError(t, err)
		require.Equal(t, false, status.InProgress)
		require.Equal(t, true, status.Successful)
		require.Equal(t, cleanupSatellite.ID(), status.SatelliteId)
	})
}
