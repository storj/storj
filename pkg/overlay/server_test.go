// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
)

func TestServer(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	// we wait a second for all the nodes to complete bootstrapping off the satellite
	time.Sleep(2 * time.Second)

	satellite := planet.Satellites[0]
	server := overlay.NewServer(satellite.Log.Named("overlay"), satellite.Overlay, &pb.NodeStats{})
	// TODO: handle cleanup

	{ // FindStorageNodes
		result, err := server.FindStorageNodes(ctx, &pb.FindStorageNodesRequest{
			Opts: &pb.OverlayOptions{Amount: 2},
		})
		require.NoError(t, err)
		require.NotNil(t, err)
		assert.Len(t, result.Nodes, 2)
	}

	{ // Lookup
		result, err := server.Lookup(ctx, &pb.LookupRequest{
			NodeId: planet.StorageNodes[0].ID(),
		})
		require.NoError(t, err)
		require.NotNil(t, err)
		assert.Equal(t, result.Node.Address.Address, planet.StorageNodes[0].Addr())
	}

	{ // BulkLookup
		result, err := server.BulkLookup(ctx, &pb.LookupRequests{
			LookupRequest: []*pb.LookupRequest{
				{NodeId: planet.StorageNodes[0].ID()},
				{NodeId: planet.StorageNodes[1].ID()},
				{NodeId: planet.StorageNodes[2].ID()},
			},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.LookupResponse, 3)

		for i, resp := range result.LookupResponse {
			if assert.NotNil(t, resp.Node) {
				assert.Equal(t, resp.Node.Address.Address, planet.StorageNodes[i].Addr())
			}
		}
	}
}
