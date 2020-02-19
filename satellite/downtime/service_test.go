// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package downtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
)

func TestCheckNodeAvailability(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		nodeDossier := planet.StorageNodes[0].Local()
		satellite := planet.Satellites[0]

		node.Contact.Chore.Pause(ctx)
		satellite.Audit.Chore.Loop.Pause()

		// test that last success and failure checks are before now
		beforeSuccessfulCheck := time.Now()
		dossier, err := satellite.Overlay.Service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.True(t, dossier.Reputation.LastContactSuccess.Before(beforeSuccessfulCheck))
		require.True(t, dossier.Reputation.LastContactFailure.Before(beforeSuccessfulCheck))

		success, err := satellite.DowntimeTracking.Service.CheckAndUpdateNodeAvailability(ctx, nodeDossier.Id, nodeDossier.Address.GetAddress())
		require.NoError(t, err)
		require.True(t, success)

		lastFailure := dossier.Reputation.LastContactFailure

		// now test that CheckAndUpdateNodeAvailability updated with a success, and the last contact failure is the same.
		dossier, err = satellite.Overlay.Service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.True(t, dossier.Reputation.LastContactSuccess.After(beforeSuccessfulCheck))
		require.True(t, dossier.Reputation.LastContactFailure.Equal(lastFailure))

		lastSuccess := dossier.Reputation.LastContactSuccess

		// shutdown the node
		err = node.Server.Close()
		require.NoError(t, err)

		// now test that CheckAndUpdateNodeAvailability updated with a failure, and the last contact success is the same
		beforeFailedCheck := time.Now()
		success, err = satellite.DowntimeTracking.Service.CheckAndUpdateNodeAvailability(ctx, nodeDossier.Id, nodeDossier.Address.GetAddress())
		require.NoError(t, err)
		require.False(t, success)

		dossier, err = satellite.Overlay.Service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.True(t, dossier.Reputation.LastContactFailure.After(beforeFailedCheck))
		require.True(t, dossier.Reputation.LastContactSuccess.Equal(lastSuccess))
	})
}
