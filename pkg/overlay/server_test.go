// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestServer(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// we wait a second for all the nodes to complete bootstrapping off the satellite
		time.Sleep(2 * time.Second)

		satellite := planet.Satellites[0]
		server := satellite.Overlay.Endpoint
		// TODO: handle cleanup

		{ // Lookup
			result, err := server.Lookup(ctx, &pb.LookupRequest{
				NodeId: planet.StorageNodes[0].ID(),
			})
			require.NoError(t, err)
			require.NotNil(t, result)
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
	})
}

func TestNodeSelection(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var err error
		satellite := planet.Satellites[0]

		// we wait a second for all the nodes to complete bootstrapping off the satellite
		time.Sleep(2 * time.Second)

		// This sets a reputable audit count for a certain number of nodes.
		for i, node := range planet.StorageNodes {
			for k := 0; k < i; k++ {
				_, err := satellite.DB.StatDB().UpdateAuditSuccess(ctx, node.ID(), true)
				assert.NoError(t, err)
			}
		}

		// ensure all storagenodes are in overlay service
		for _, storageNode := range planet.StorageNodes {
			err = satellite.Overlay.Service.Put(ctx, storageNode.ID(), storageNode.Local())
			assert.NoError(t, err)
		}

		type test struct {
			Preferences    overlay.NodeSelectionConfig
			ExcludeCount   int
			RequestCount   int64
			ExpectedCount  int
			ShouldFailWith *errs.Class
		}

		for i, tt := range []test{
			{ // all reputable nodes, only reputable nodes requested
				Preferences: overlay.NodeSelectionConfig{
					NewNodeAuditThreshold: 0,
					NewNodePercentage:     0,
				},
				RequestCount:  5,
				ExpectedCount: 5,
			},
			{ // all reputable nodes, reputable and new nodes requested
				Preferences: overlay.NodeSelectionConfig{
					NewNodeAuditThreshold: 0,
					NewNodePercentage:     1,
				},
				RequestCount:  5,
				ExpectedCount: 5,
			},
			{ // all reputable nodes except one, reputable and new nodes requested
				Preferences: overlay.NodeSelectionConfig{
					NewNodeAuditThreshold: 1,
					NewNodePercentage:     1,
				},
				RequestCount:  5,
				ExpectedCount: 6,
			},
			{ // 50-50 reputable and new nodes, reputable and new nodes requested (new node ratio 1.0)
				Preferences: overlay.NodeSelectionConfig{
					NewNodeAuditThreshold: 5,
					NewNodePercentage:     1,
				},
				RequestCount:  2,
				ExpectedCount: 4,
			},
			{ // 50-50 reputable and new nodes, reputable and new nodes requested (new node ratio 0.5)
				Preferences: overlay.NodeSelectionConfig{
					NewNodeAuditThreshold: 5,
					NewNodePercentage:     0.5,
				},
				RequestCount:  4,
				ExpectedCount: 6,
			},
			{ // all new nodes except one, reputable and new nodes requested (happy path)
				Preferences: overlay.NodeSelectionConfig{
					NewNodeAuditThreshold: 8,
					NewNodePercentage:     1,
				},
				RequestCount:  1,
				ExpectedCount: 2,
			},
			{ // all new nodes except one, reputable and new nodes requested (not happy path)
				Preferences: overlay.NodeSelectionConfig{
					NewNodeAuditThreshold: 9,
					NewNodePercentage:     1,
				},
				RequestCount:   2,
				ExpectedCount:  3,
				ShouldFailWith: &overlay.ErrNotEnoughNodes,
			},
			{ // all new nodes, reputable and new nodes requested
				Preferences: overlay.NodeSelectionConfig{
					NewNodeAuditThreshold: 50,
					NewNodePercentage:     1,
				},
				RequestCount:   2,
				ExpectedCount:  2,
				ShouldFailWith: &overlay.ErrNotEnoughNodes,
			},
			{ // audit threshold edge case (1)
				Preferences: overlay.NodeSelectionConfig{
					NewNodeAuditThreshold: 9,
					NewNodePercentage:     0,
				},
				RequestCount:  1,
				ExpectedCount: 1,
			},
			{ // audit threshold edge case (2)
				Preferences: overlay.NodeSelectionConfig{
					NewNodeAuditThreshold: 0,
					NewNodePercentage:     1,
				},
				RequestCount:  1,
				ExpectedCount: 1,
			},
			{ // excluded node ids being excluded
				Preferences: overlay.NodeSelectionConfig{
					NewNodeAuditThreshold: 5,
					NewNodePercentage:     0,
				},
				ExcludeCount:   7,
				RequestCount:   5,
				ExpectedCount:  3,
				ShouldFailWith: &overlay.ErrNotEnoughNodes,
			},
		} {
			t.Logf("#%2d. %+v", i, tt)
			endpoint := planet.Satellites[0].Overlay.Endpoint

			var excludedNodes []storj.NodeID
			for _, storageNode := range planet.StorageNodes[:tt.ExcludeCount] {
				excludedNodes = append(excludedNodes, storageNode.ID())
			}

			response, err := endpoint.FindStorageNodesWithPreferences(ctx,
				&pb.FindStorageNodesRequest{
					Opts: &pb.OverlayOptions{
						Restrictions: &pb.NodeRestrictions{
							FreeBandwidth: 0,
							FreeDisk:      0,
						},
						Amount:        tt.RequestCount,
						ExcludedNodes: excludedNodes,
					},
				}, &tt.Preferences)

			t.Log(len(response.Nodes), err)
			if tt.ShouldFailWith != nil {
				assert.Error(t, err)
				assert.True(t, tt.ShouldFailWith.Has(err))
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.ExpectedCount, len(response.Nodes))
		}
	})
}
