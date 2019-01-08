// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	server := overlay.NewServer(satellite.Log.Named("overlay"), satellite.Overlay, &pb.NodeStats{}, 2, 0, 0)
	// TODO: handle cleanup

	{ // FindStorageNodes
		result, err := server.FindStorageNodes(ctx, &pb.FindStorageNodesRequest{Opts: &pb.OverlayOptions{Amount: 2}})
		if assert.NoError(t, err) && assert.NotNil(t, result) {
			assert.Len(t, result.Nodes, 2)
		}
	}

	{ // Lookup
		result, err := server.Lookup(ctx, &pb.LookupRequest{NodeId: planet.StorageNodes[0].ID()})
		if assert.NoError(t, err) && assert.NotNil(t, result) {
			assert.Equal(t, result.Node.Address.Address, planet.StorageNodes[0].Addr())
		}
	}

	{ // BulkLookup
		result, err := server.BulkLookup(ctx, &pb.LookupRequests{
			LookupRequest: []*pb.LookupRequest{
				{NodeId: planet.StorageNodes[0].ID()},
				{NodeId: planet.StorageNodes[1].ID()},
				{NodeId: planet.StorageNodes[2].ID()},
			},
		})

		if assert.NoError(t, err) && assert.NotNil(t, result) && assert.Len(t, result.LookupResponse, 3) {
			for i, resp := range result.LookupResponse {
				if assert.NotNil(t, resp.Node) {
					assert.Equal(t, resp.Node.Address.Address, planet.StorageNodes[i].Addr())
				}
			}
		}
	}
}

func TestFewerThanRequiredReputableNodes(t *testing.T) {
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

	// todo(nat):
	// create a gateway to upload files to the test nodes
	// then make sure that the right number of nodes are audited
	// then confirm right outputs

	// gateway := miniogw.NewStorjGateway(storj.Metainfo{}, streams.Store{}, storj.Cipher{}, storj.EncryptionScheme{}, storj.RedundancySceme{})

	satellite := planet.Satellites[0]
	server := overlay.NewServer(satellite.Log.Named("overlay"), satellite.Overlay, &pb.NodeStats{}, 2, 1, 0.5)

	result, err := server.FindStorageNodes(ctx,
		&pb.FindStorageNodesRequest{
			Opts: &pb.OverlayOptions{Amount: 2},
		})
	stat, ok := status.FromError(err)
	assert.Equal(t, true, ok)
	assert.Equal(t, codes.ResourceExhausted, stat.Code)
	assert.Equal(t, 3, len(result.GetNodes()))
}

// other tests:
// 	more than required reputable nodes
// 	zero reputable nodes found, only new nodes
// 	fewer than required new nodes
// 	more than required new nodes
// 	zero new nodes found, only reputable nodes
// 	exactly the required amount of new and reputable nodes returned
// 	low percentage of new nodes
// 	high percentage of new nodes
// 	0% new nodes requested
