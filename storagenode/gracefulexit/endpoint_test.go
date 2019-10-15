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

func TestInitiateGracefulExit(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	totalSatelliteCount := 3
	planet, err := testplanet.New(t, totalSatelliteCount, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	storagenode := planet.StorageNodes[0]

	exitingSatelliteID := planet.Satellites[0].ID()

	req := &pb.InitiateGracefulExitRequest{
		NodeId: exitingSatelliteID,
	}

	resp, err := storagenode.GracefulExit.Endpoint.InitiateGracefulExit(ctx, req)
	require.NoError(t, err)
	// check progress is 0
	require.EqualValues(t, 0, resp.GetPercentComplete())
	require.False(t, resp.GetSuccessful())

	exitStatuses, err := storagenode.DB.Satellites().ListGracefulExits(ctx)
	require.NoError(t, err)
	require.Len(t, exitStatuses, 1)
	require.Equal(t, exitingSatelliteID, exitStatuses[0].SatelliteID)
}

func TestGetExitProgress(t *testing.T) {
	ctx := testcontext.New(t)

	totalSatelliteCount := 3
	planet, err := testplanet.New(t, totalSatelliteCount, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	exitingSatellite := planet.Satellites[0]
	storagenode := planet.StorageNodes[0]

	// start graceful exit
	err = storagenode.DB.Satellites().InitiateGracefulExit(ctx, exitingSatellite.ID(), time.Now().UTC(), 100)
	require.NoError(t, err)
	err = storagenode.DB.Satellites().UpdateGracefulExit(ctx, exitingSatellite.ID(), 20)
	require.NoError(t, err)

	// check graceful exit progress
	resp, err := storagenode.GracefulExit.Endpoint.GetExitProgress(ctx, &pb.GetExitProgressRequest{})
	require.NoError(t, err)
	require.Len(t, resp.GetProgress(), 1)
	progress := resp.GetProgress()[0]
	require.Equal(t, progress.GetDomainName(), exitingSatellite.Addr())
	require.Equal(t, progress.NodeId, exitingSatellite.ID())
	require.EqualValues(t, 20, progress.GetPercentComplete())
}
