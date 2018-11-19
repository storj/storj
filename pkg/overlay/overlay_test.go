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
	t.Skip("Not working right now.")

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	overlay, err := planet.Uplinks[0].DialOverlay(planet.Satellites[0])
	if err != nil {
		t.Fatal(err)
	}

	{ // FindStorageNodes
		result, err := overlay.FindStorageNodes(ctx, &pb.FindStorageNodesRequest{Opts: &pb.OverlayOptions{Amount: 2}})
		if assert.NoError(t, err) && assert.NotNil(t, result) {
			assert.Len(t, result.Nodes, 2)
		}
	}

	{ // Lookup
		result, err := overlay.Lookup(ctx, &pb.LookupRequest{NodeID: planet.StorageNodes[0].ID()})
		if assert.NoError(t, err) && assert.NotNil(t, result) {
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

		if assert.NoError(t, err) && assert.NotNil(t, result) && assert.Len(t, result.Lookupresponse, 3) {
			for i, resp := range result.Lookupresponse {
				if assert.NotNil(t, resp.Node) {
					assert.Equal(t, resp.Node.Address.Address, planet.StorageNodes[i].Addr())
				}
			}
		}
	}
}
