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

func TestNewNodeFiltering(t *testing.T) {
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

	for i, tt := range []struct {
		name                  string
		newNodeAuditThreshold int64
		newNodePercentage     float64
		requestedNodeAmt      int64
		expectedResultLength  int
		reputableNodes        int
	}{
		{
			name:                  "case: fewer than required reputable nodes",
			requestedNodeAmt:      4,
			reputableNodes:        3,
			expectedResultLength:  3,
			newNodeAuditThreshold: 1,
		},
		{
			name:                  "case: more than required reputable nodes",
			requestedNodeAmt:      2,
			reputableNodes:        3,
			expectedResultLength:  2,
			newNodeAuditThreshold: 1,
		},
		{
			name:                  "case: zero reputable nodes found, only new nodes",
			requestedNodeAmt:      2,
			reputableNodes:        0,
			expectedResultLength:  2,
			newNodeAuditThreshold: 1,
		},
		{
			name:              "case: fewer than required new nodes",
			requestedNodeAmt:  2,
			reputableNodes:    3,
			newNodePercentage: 0.5,
			// this gives extra reputable instead
			expectedResultLength:  3,
			newNodeAuditThreshold: 1,
		},
		{
			name:                  "case: more than required new nodes",
			requestedNodeAmt:      2,
			reputableNodes:        2,
			newNodePercentage:     0.5,
			expectedResultLength:  3,
			newNodeAuditThreshold: 1,
		},
		{
			// todo(nat): fix nodes length issue
			name:                  "case: zero new nodes found, only reputable nodes",
			requestedNodeAmt:      3,
			reputableNodes:        3,
			newNodePercentage:     0.5,
			expectedResultLength:  4,
			newNodeAuditThreshold: 1,
		},
		{
			name:                  "case: exactly the required amount of new and reputable nodes returned",
			requestedNodeAmt:      1,
			reputableNodes:        1,
			newNodePercentage:     1,
			expectedResultLength:  2,
			newNodeAuditThreshold: 1,
		},
		{
			name:              "case: low percentage of new nodes",
			requestedNodeAmt:  3,
			reputableNodes:    1,
			newNodePercentage: 0.01,
			// todo(nat): expect this result to be 1
			expectedResultLength:  3,
			newNodeAuditThreshold: 1,
		},
		{
			name:                  "case: high percentage of new nodes",
			requestedNodeAmt:      1,
			reputableNodes:        1,
			newNodePercentage:     3,
			expectedResultLength:  4,
			newNodeAuditThreshold: 1,
		},
		{
			name:                  "case: 0% new nodes requested",
			requestedNodeAmt:      1,
			reputableNodes:        1,
			newNodePercentage:     0,
			expectedResultLength:  1,
			newNodeAuditThreshold: 1,
		},
	} {
		server := overlay.NewServer(satellite.Log.Named("overlay"), satellite.Overlay,
			&pb.NodeStats{}, 2, tt.newNodeAuditThreshold, tt.newNodePercentage)

		for i := 0; i <= tt.reputableNodes; i++ {
			satellite.Overlay.Put(ctx, planet.StorageNodes[i].ID(), pb.Node{
				Reputation: &pb.NodeStats{AuditCount: 1},
			})
		}

		result, err := server.FindStorageNodes(ctx,
			&pb.FindStorageNodesRequest{
				Opts: &pb.OverlayOptions{Amount: tt.requestedNodeAmt},
			})

		if i == 0 {
			stat, ok := status.FromError(err)
			assert.Equal(t, true, ok, tt.name)
			assert.Equal(t, codes.ResourceExhausted, stat.Code(), tt.name)
		} else {
			assert.NoError(t, err, tt.name)
		}
		assert.Equal(t, tt.expectedResultLength, len(result.GetNodes()), tt.name)

		// resetting audit count to 0
		for i := 0; i <= tt.reputableNodes; i++ {
			satellite.Overlay.Put(ctx, planet.StorageNodes[i].ID(), pb.Node{
				Reputation: &pb.NodeStats{AuditCount: 0},
			})
		}
	}
}
