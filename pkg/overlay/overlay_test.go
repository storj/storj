// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
)

func TestOverlay(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	overlay, err := planet.Uplinks[0].DialOverlay(planet.Satellites[0])
	if err != nil {
		t.Fatal(err)
	}

	{ // FindStorageNodes
		result, err := overlay.FindStorageNodes(ctx, &pb.FindStorageNodesRequest{Opts: &pb.OverlayOptions{Amount: 2}})
		if assert.NoError(t, err) {
			assert.NotNil(t, result)
			assert.Len(t, result.Nodes, 2)
		}
	}

	{ // Lookup
		result, err := overlay.Lookup(ctx, &pb.LookupRequest{NodeID: planet.StorageNodes[0].ID()})
		if assert.NoError(t, err) {
			assert.NotNil(t, result)
			assert.Equal(t, result.Node.Address.Address, planet.StorageNodes[0].Addr())
		}
	}

	{ // BulkLookup
		result, err := overlay.BulkLookup(ctx, &pb.LookupRequests{
			Lookuprequest: []*pb.LookupRequest{
				{NodeID: planet.StorageNodes[0].ID()},
				{NodeID: planet.StorageNodes[1].ID()},
				{NodeID: planet.StorageNodes[2].ID()},
			},
		})

		if assert.NoError(t, err) {
			assert.NotNil(t, result)
			assert.Len(t, result.Lookupresponse, 3)
			assert.Equal(t, result.Lookupresponse[0].Node.Address.Address, planet.StorageNodes[0].Addr())
			assert.Equal(t, result.Lookupresponse[1].Node.Address.Address, planet.StorageNodes[1].Addr())
			assert.Equal(t, result.Lookupresponse[2].Node.Address.Address, planet.StorageNodes[2].Addr())
		}
	}
}
