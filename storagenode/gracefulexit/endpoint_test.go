// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestGetNonExitingSatellites(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	totalSatelliteCount := 3
	exitingSatelliteCount := 1
	planet, err := testplanet.New(t, totalSatelliteCount, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	exitingSatellite := planet.Satellites[0]
	storagenode := planet.StorageNodes[0]

	// set a satellite to already be exiting
	err = storagenode.DB.Satellites().InitiateGracefulExit(ctx, exitingSatellite.ID(), time.Now().UTC(), 0)
	require.NoError(t, err)

	nonExitingSatellites, err := storagenode.GracefulExit.Endpoint.GetNonExitingSatellites(ctx, &pb.GetNonExitingSatellitesRequest{})
	require.NoError(t, err)
	require.Len(t, nonExitingSatellites.GetSatellites(), totalSatelliteCount-exitingSatelliteCount)

	for _, satellite := range nonExitingSatellites.GetSatellites() {
		require.NotEqual(t, exitingSatellite.ID(), satellite.NodeId)
	}
}

func TestStartExiting(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	totalSatelliteCount := 3
	const exitingSatelliteCount = 2
	planet, err := testplanet.New(t, totalSatelliteCount, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	storagenode := planet.StorageNodes[0]

	exitingSatelliteIDs := []storj.NodeID{
		planet.Satellites[0].ID(),
		planet.Satellites[1].ID(),
	}
	req := &pb.StartExitRequest{
		NodeIds: exitingSatelliteIDs,
	}

	resp, err := storagenode.GracefulExit.Endpoint.StartExit(ctx, req)
	require.NoError(t, err)
	for _, status := range resp.GetStatuses() {
		require.True(t, status.GetSuccess())
	}

	exitStatuses, err := storagenode.DB.Satellites().ListGracefulExits(ctx)
	require.NoError(t, err)
	require.Len(t, exitStatuses, exitingSatelliteCount)
	for _, status := range exitStatuses {
		require.Contains(t, exitingSatelliteIDs, status.SatelliteID)
	}
}
