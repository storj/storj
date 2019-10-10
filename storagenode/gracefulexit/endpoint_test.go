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

func TestGetNonExistingSatellites(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	totalSatelliteCount := 3
	existingSatelliteCount := 1
	planet, err := testplanet.New(t, totalSatelliteCount, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	exitingSatellite := planet.Satellites[0]
	storagenode := planet.StorageNodes[0]

	// set a satellite to already be exiting
	err = storagenode.DB.Satellites().InitiateGracefulExit(ctx, exitingSatellite.ID(), time.Now().UTC(), 0)
	require.NoError(t, err)

	nonExistingSatellites, err := storagenode.GracefulExit.Endpoint.GetNonExitingSatellites(ctx, &pb.GetNonExitingSatellitesRequest{})
	require.NoError(t, err)
	require.Len(t, nonExistingSatellites.GetSatellites(), totalSatelliteCount-existingSatelliteCount)
}

func TestStartExiting(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	totalSatelliteCount := 3
	const existingSatelliteCount = 2
	planet, err := testplanet.New(t, totalSatelliteCount, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	storagenode := planet.StorageNodes[0]

	exitingSatellites := [existingSatelliteCount]*testplanet.SatelliteSystem{
		planet.Satellites[0],
		planet.Satellites[1],
	}

	req := &pb.StartExitRequest{
		NodeIds: []storj.NodeID{
			exitingSatellites[0].ID(),
			exitingSatellites[1].ID(),
		},
	}

	resp, err := storagenode.GracefulExit.Endpoint.StartExit(ctx, req)
	require.NoError(t, err)
	for _, status := range resp.GetStatuses() {
		require.True(t, status.GetSuccess())
	}
}
