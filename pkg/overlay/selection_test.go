// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
)

func TestOffline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		service := satellite.Overlay.Service
		// TODO: handle cleanup

		result, err := service.KnownUnreliableOrOffline(ctx, []storj.NodeID{
			planet.StorageNodes[0].ID(),
		})
		require.NoError(t, err)
		require.Empty(t, result)

		result, err = service.KnownUnreliableOrOffline(ctx, []storj.NodeID{
			planet.StorageNodes[0].ID(),
			planet.StorageNodes[1].ID(),
			planet.StorageNodes[2].ID(),
		})
		require.NoError(t, err)
		require.Empty(t, result)

		result, err = service.KnownUnreliableOrOffline(ctx, []storj.NodeID{
			planet.StorageNodes[0].ID(),
			storj.NodeID{1, 2, 3, 4}, //note that this succeeds by design
			planet.StorageNodes[2].ID(),
		})
		require.NoError(t, err)
		require.Len(t, result, 0)
	})
}

func TestNodeSelection(t *testing.T) {
	t.Skip("flaky")
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var err error
		satellite := planet.Satellites[0]

		// This sets a reputable audit count for a certain number of nodes.
		for i, node := range planet.StorageNodes {
			for k := 0; k < i; k++ {
				_, err := satellite.DB.OverlayCache().UpdateStats(ctx, &overlay.UpdateRequest{
					NodeID:       node.ID(),
					IsUp:         true,
					AuditSuccess: true,
				})
				assert.NoError(t, err)
			}
		}

		// ensure all storagenodes are in overlay service
		for _, storageNode := range planet.StorageNodes {
			err = satellite.Overlay.Service.Put(ctx, storageNode.ID(), storageNode.Local().Node)
			assert.NoError(t, err)
		}

		type test struct {
			Preferences    overlay.NodeSelectionConfig
			ExcludeCount   int
			RequestCount   int
			ExpectedCount  int
			ShouldFailWith *errs.Class
		}

		for i, tt := range []test{
			{ // all reputable nodes, only reputable nodes requested
				Preferences: overlay.NodeSelectionConfig{
					AuditCount:        0,
					NewNodePercentage: 0,
					OnlineWindow:      time.Hour,
				},
				RequestCount:  5,
				ExpectedCount: 5,
			},
			{ // all reputable nodes, reputable and new nodes requested
				Preferences: overlay.NodeSelectionConfig{
					AuditCount:        0,
					NewNodePercentage: 1,
					OnlineWindow:      time.Hour,
				},
				RequestCount:  5,
				ExpectedCount: 5,
			},
			{ // all reputable nodes except one, reputable and new nodes requested
				Preferences: overlay.NodeSelectionConfig{
					AuditCount:        1,
					NewNodePercentage: 1,
					OnlineWindow:      time.Hour,
				},
				RequestCount:  5,
				ExpectedCount: 6,
			},
			{ // 50-50 reputable and new nodes, reputable and new nodes requested (new node ratio 1.0)
				Preferences: overlay.NodeSelectionConfig{
					AuditCount:        5,
					NewNodePercentage: 1,
					OnlineWindow:      time.Hour,
				},
				RequestCount:  2,
				ExpectedCount: 4,
			},
			{ // 50-50 reputable and new nodes, reputable and new nodes requested (new node ratio 0.5)
				Preferences: overlay.NodeSelectionConfig{
					AuditCount:        5,
					NewNodePercentage: 0.5,
					OnlineWindow:      time.Hour,
				},
				RequestCount:  4,
				ExpectedCount: 6,
			},
			{ // all new nodes except one, reputable and new nodes requested (happy path)
				Preferences: overlay.NodeSelectionConfig{
					AuditCount:        8,
					NewNodePercentage: 1,
					OnlineWindow:      time.Hour,
				},
				RequestCount:  1,
				ExpectedCount: 2,
			},
			{ // all new nodes except one, reputable and new nodes requested (not happy path)
				Preferences: overlay.NodeSelectionConfig{
					AuditCount:        9,
					NewNodePercentage: 1,
					OnlineWindow:      time.Hour,
				},
				RequestCount:   2,
				ExpectedCount:  3,
				ShouldFailWith: &overlay.ErrNotEnoughNodes,
			},
			{ // all new nodes, reputable and new nodes requested
				Preferences: overlay.NodeSelectionConfig{
					AuditCount:        50,
					NewNodePercentage: 1,
					OnlineWindow:      time.Hour,
				},
				RequestCount:   2,
				ExpectedCount:  2,
				ShouldFailWith: &overlay.ErrNotEnoughNodes,
			},
			{ // audit threshold edge case (1)
				Preferences: overlay.NodeSelectionConfig{
					AuditCount:        9,
					NewNodePercentage: 0,
					OnlineWindow:      time.Hour,
				},
				RequestCount:  1,
				ExpectedCount: 1,
			},
			{ // audit threshold edge case (2)
				Preferences: overlay.NodeSelectionConfig{
					AuditCount:        0,
					NewNodePercentage: 1,
					OnlineWindow:      time.Hour,
				},
				RequestCount:  1,
				ExpectedCount: 1,
			},
			{ // excluded node ids being excluded
				Preferences: overlay.NodeSelectionConfig{
					AuditCount:        5,
					NewNodePercentage: 0,
					OnlineWindow:      time.Hour,
				},
				ExcludeCount:   7,
				RequestCount:   5,
				ExpectedCount:  3,
				ShouldFailWith: &overlay.ErrNotEnoughNodes,
			},
		} {
			t.Logf("#%2d. %+v", i, tt)
			service := planet.Satellites[0].Overlay.Service

			var excludedNodes []storj.NodeID
			for _, storageNode := range planet.StorageNodes[:tt.ExcludeCount] {
				excludedNodes = append(excludedNodes, storageNode.ID())
			}

			response, err := service.FindStorageNodesWithPreferences(ctx, overlay.FindStorageNodesRequest{
				FreeBandwidth:  0,
				FreeDisk:       0,
				RequestedCount: tt.RequestCount,
				ExcludedNodes:  excludedNodes,
			}, &tt.Preferences)

			t.Log(len(response), err)
			if tt.ShouldFailWith != nil {
				assert.Error(t, err)
				assert.True(t, tt.ShouldFailWith.Has(err))
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.ExpectedCount, len(response))
		}
	})
}

func TestDistinctIPs(t *testing.T) {
	tests := []struct {
		nodeCount      int
		duplicateCount int
		requestCount   int
		preferences    overlay.NodeSelectionConfig
	}{
		{ // test only distinct IPs are retrieved
			nodeCount:      10,
			duplicateCount: 7,
			requestCount:   4,
			preferences: overlay.NodeSelectionConfig{
				AuditCount:        0,
				NewNodePercentage: 0,
				OnlineWindow:      time.Hour,
				DistinctIP:        true,
			},
		},
		{ // test distinct flag false allows duplicates
			nodeCount:      10,
			duplicateCount: 10,
			requestCount:   5,
			preferences: overlay.NodeSelectionConfig{
				AuditCount:        0,
				NewNodePercentage: 0,
				OnlineWindow:      time.Hour,
				DistinctIP:        false,
			},
		},
	}

	for _, tt := range tests {
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: tt.nodeCount, UplinkCount: 1,
			Reconfigure: testplanet.Reconfigure{
				Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
					config.Overlay.Node.DistinctIP = tt.preferences.DistinctIP
					config.Discovery.RefreshInterval = 60 * time.Second
				},
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			var err error
			service := planet.Satellites[0].Overlay.Service
			time.Sleep(1 * time.Second)

			// update node last IPs
			for i := 0; i < tt.nodeCount; i++ {
				node := planet.StorageNodes[i].Local().Node
				if i < tt.duplicateCount {
					node.LastIp = "127.0.0.1"
				} else {
					node.LastIp = strconv.Itoa(i)
				}

				err = service.Put(ctx, planet.StorageNodes[i].ID(), node)
				require.NoError(t, err)
			}

			response, err := service.FindStorageNodesWithPreferences(ctx, overlay.FindStorageNodesRequest{
				FreeBandwidth:  0,
				FreeDisk:       0,
				RequestedCount: tt.requestCount,
			}, &tt.preferences)
			require.NoError(t, err)

			// assert all IPs are unique
			if tt.preferences.DistinctIP {
				ips := make(map[string]bool)
				for _, n := range response {
					fmt.Println(n.Id, n.LastIp)
					assert.False(t, ips[n.LastIp])
					ips[n.LastIp] = true
				}
			}

			assert.Equal(t, tt.requestCount, len(response))
		})
	}
}
