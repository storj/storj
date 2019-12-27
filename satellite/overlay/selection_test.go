// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
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
			{1, 2, 3, 4}, //note that this succeeds by design
			planet.StorageNodes[2].ID(),
		})
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, result[0], storj.NodeID{1, 2, 3, 4})
	})
}

func TestNodeSelection(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// This sets audit counts of 0, 1, 2, 3, ... 9
		// so that we can fine-tune how many nodes are considered new or reputable
		// by modifying the audit count cutoff passed into FindStorageNodesWithPreferences
		for i, node := range planet.StorageNodes {
			for k := 0; k < i; k++ {
				_, err := satellite.DB.OverlayCache().UpdateStats(ctx, &overlay.UpdateRequest{
					NodeID:       node.ID(),
					IsUp:         true,
					AuditSuccess: true,
					AuditLambda:  1, AuditWeight: 1, AuditDQ: 0.5,
					UptimeLambda: 1, UptimeWeight: 1, UptimeDQ: 0.5,
				})
				require.NoError(t, err)
			}
		}
		testNodeSelection(t, ctx, planet)
	})
}

func TestNodeSelectionWithBatch(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// This sets audit counts of 0, 1, 2, 3, ... 9
		// so that we can fine-tune how many nodes are considered new or reputable
		// by modifying the audit count cutoff passed into FindStorageNodesWithPreferences
		for i, node := range planet.StorageNodes {
			for k := 0; k < i; k++ {
				// These are done individually b/c the previous stat data is important
				_, err := satellite.DB.OverlayCache().BatchUpdateStats(ctx, []*overlay.UpdateRequest{{
					NodeID:       node.ID(),
					IsUp:         true,
					AuditSuccess: true,
					AuditLambda:  1, AuditWeight: 1, AuditDQ: 0.5,
					UptimeLambda: 1, UptimeWeight: 1, UptimeDQ: 0.5,
				}}, 1)
				require.NoError(t, err)
			}
		}
		testNodeSelection(t, ctx, planet)
	})
}

func testNodeSelection(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
	satellite := planet.Satellites[0]
	// ensure all storagenodes are in overlay
	for _, storageNode := range planet.StorageNodes {
		err := satellite.Overlay.Service.Put(ctx, storageNode.ID(), storageNode.Local().Node)
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
			Preferences:   testNodeSelectionConfig(0, 0, false),
			RequestCount:  5,
			ExpectedCount: 5,
		},
		{ // all reputable nodes, reputable and new nodes requested
			Preferences:   testNodeSelectionConfig(0, 1, false),
			RequestCount:  5,
			ExpectedCount: 5,
		},
		{ // 50-50 reputable and new nodes, not enough reputable nodes
			Preferences:    testNodeSelectionConfig(5, 0, false),
			RequestCount:   10,
			ExpectedCount:  5,
			ShouldFailWith: &overlay.ErrNotEnoughNodes,
		},
		{ // 50-50 reputable and new nodes, reputable and new nodes requested, not enough reputable nodes
			Preferences:    testNodeSelectionConfig(5, 0.2, false),
			RequestCount:   10,
			ExpectedCount:  7,
			ShouldFailWith: &overlay.ErrNotEnoughNodes,
		},
		{ // all new nodes except one, reputable and new nodes requested (happy path)
			Preferences:   testNodeSelectionConfig(9, 0.5, false),
			RequestCount:  2,
			ExpectedCount: 2,
		},
		{ // all new nodes except one, reputable and new nodes requested (not happy path)
			Preferences:    testNodeSelectionConfig(9, 0.5, false),
			RequestCount:   4,
			ExpectedCount:  3,
			ShouldFailWith: &overlay.ErrNotEnoughNodes,
		},
		{ // all new nodes, reputable and new nodes requested
			Preferences:   testNodeSelectionConfig(50, 1, false),
			RequestCount:  2,
			ExpectedCount: 2,
		},
		{ // audit threshold edge case (1)
			Preferences:   testNodeSelectionConfig(9, 0, false),
			RequestCount:  1,
			ExpectedCount: 1,
		},
		{ // excluded node ids being excluded
			Preferences:    testNodeSelectionConfig(5, 0, false),
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
}

func TestNodeSelectionGracefulExit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		exitingNodes := make(map[storj.NodeID]bool)

		// This sets audit counts of 0, 1, 2, 3, ... 9
		// so that we can fine-tune how many nodes are considered new or reputable
		// by modifying the audit count cutoff passed into FindStorageNodesWithPreferences
		// nodes at indices 0, 2, 4, 6, 8 are gracefully exiting
		for i, node := range planet.StorageNodes {
			for k := 0; k < i; k++ {
				_, err := satellite.DB.OverlayCache().UpdateStats(ctx, &overlay.UpdateRequest{
					NodeID:       node.ID(),
					IsUp:         true,
					AuditSuccess: true,
					AuditLambda:  1, AuditWeight: 1, AuditDQ: 0.5,
					UptimeLambda: 1, UptimeWeight: 1, UptimeDQ: 0.5,
				})
				require.NoError(t, err)
			}

			// make half the nodes gracefully exiting
			if i%2 == 0 {
				_, err := satellite.DB.OverlayCache().UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
					NodeID:          node.ID(),
					ExitInitiatedAt: time.Now(),
				})
				require.NoError(t, err)
				exitingNodes[node.ID()] = true
			}
		}

		type test struct {
			Preferences    overlay.NodeSelectionConfig
			ExcludeCount   int
			RequestCount   int
			ExpectedCount  int
			ShouldFailWith *errs.Class
		}

		for i, tt := range []test{
			{ // reputable and new nodes, happy path
				Preferences:   testNodeSelectionConfig(5, 0.5, false),
				RequestCount:  5,
				ExpectedCount: 5,
			},
			{ // all reputable nodes, happy path
				Preferences:   testNodeSelectionConfig(0, 1, false),
				RequestCount:  5,
				ExpectedCount: 5,
			},
			{ // all new nodes, happy path
				Preferences:   testNodeSelectionConfig(50, 1, false),
				RequestCount:  5,
				ExpectedCount: 5,
			},
			{ // reputable and new nodes, requested too many
				Preferences:    testNodeSelectionConfig(5, 0.5, false),
				RequestCount:   10,
				ExpectedCount:  5,
				ShouldFailWith: &overlay.ErrNotEnoughNodes,
			},
			{ // all reputable nodes, requested too many
				Preferences:    testNodeSelectionConfig(0, 1, false),
				RequestCount:   10,
				ExpectedCount:  5,
				ShouldFailWith: &overlay.ErrNotEnoughNodes,
			},
			{ // all new nodes, requested too many
				Preferences:    testNodeSelectionConfig(50, 1, false),
				RequestCount:   10,
				ExpectedCount:  5,
				ShouldFailWith: &overlay.ErrNotEnoughNodes,
			},
		} {
			t.Logf("#%2d. %+v", i, tt)

			response, err := satellite.Overlay.Service.FindStorageNodesWithPreferences(ctx, overlay.FindStorageNodesRequest{
				FreeBandwidth:  0,
				FreeDisk:       0,
				RequestedCount: tt.RequestCount,
			}, &tt.Preferences)

			t.Log(len(response), err)
			if tt.ShouldFailWith != nil {
				assert.Error(t, err)
				assert.True(t, tt.ShouldFailWith.Has(err))
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.ExpectedCount, len(response))

			// expect no exiting nodes in selection
			for _, node := range response {
				assert.False(t, exitingNodes[node.Id])
			}
		}
	})
}

func TestFindStorageNodesDistinctIPs(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Test does not work with macOS")
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			// will create 3 storage nodes with same IP; 2 will have unique
			UniqueIPCount: 2,
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.DistinctIP = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// select one of the nodes that shares an IP with others to exclude
		var excludedNodes storj.NodeIDList
		addrCounts := make(map[string]int)
		var excludedNodeAddr string
		for _, node := range planet.StorageNodes {
			addrNoPort := strings.Split(node.Addr(), ":")[0]
			if addrCounts[addrNoPort] > 0 && len(excludedNodes) == 0 {
				excludedNodes = append(excludedNodes, node.ID())
				break
			}
			addrCounts[addrNoPort]++
		}
		require.Len(t, excludedNodes, 1)
		res, err := satellite.Overlay.Service.Get(ctx, excludedNodes[0])
		require.NoError(t, err)
		excludedNodeAddr = res.LastIp

		req := overlay.FindStorageNodesRequest{
			MinimumRequiredNodes: 2,
			RequestedCount:       2,
			ExcludedNodes:        excludedNodes,
		}
		nodes, err := satellite.Overlay.Service.FindStorageNodes(ctx, req)
		require.NoError(t, err)
		require.Len(t, nodes, 2)
		require.NotEqual(t, nodes[0].LastIp, nodes[1].LastIp)
		require.NotEqual(t, nodes[0].LastIp, excludedNodeAddr)
		require.NotEqual(t, nodes[1].LastIp, excludedNodeAddr)

		req = overlay.FindStorageNodesRequest{
			MinimumRequiredNodes: 3,
			RequestedCount:       3,
			ExcludedNodes:        excludedNodes,
		}
		_, err = satellite.Overlay.Service.FindStorageNodes(ctx, req)
		require.Error(t, err)
	})
}

func TestDistinctIPs(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Test does not work with macOS")
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			UniqueIPCount: 3,
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		// This sets a reputable audit count for nodes[8] and nodes[9].
		for i := 9; i > 7; i-- {
			_, err := satellite.DB.OverlayCache().UpdateStats(ctx, &overlay.UpdateRequest{
				NodeID:       planet.StorageNodes[i].ID(),
				IsUp:         true,
				AuditSuccess: true,
				AuditLambda:  1,
				AuditWeight:  1,
				AuditDQ:      0.5,
				UptimeLambda: 1,
				UptimeWeight: 1,
				UptimeDQ:     0.5,
			})
			assert.NoError(t, err)
		}
		testDistinctIPs(t, ctx, planet)
	})
}

func TestDistinctIPsWithBatch(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Test does not work with macOS")
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			UniqueIPCount: 3,
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		// This sets a reputable audit count for nodes[8] and nodes[9].
		for i := 9; i > 7; i-- {
			// These are done individually b/c the previous stat data is important
			_, err := satellite.DB.OverlayCache().BatchUpdateStats(ctx, []*overlay.UpdateRequest{{
				NodeID:       planet.StorageNodes[i].ID(),
				IsUp:         true,
				AuditSuccess: true,
				AuditLambda:  1,
				AuditWeight:  1,
				AuditDQ:      0.5,
				UptimeLambda: 1,
				UptimeWeight: 1,
				UptimeDQ:     0.5,
			}}, 1)
			assert.NoError(t, err)
		}
		testDistinctIPs(t, ctx, planet)
	})
}

func testDistinctIPs(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
	satellite := planet.Satellites[0]
	service := satellite.Overlay.Service

	tests := []struct {
		duplicateCount int
		requestCount   int
		preferences    overlay.NodeSelectionConfig
		shouldFailWith *errs.Class
	}{
		{ // test only distinct IPs with half new nodes
			requestCount: 4,
			preferences:  testNodeSelectionConfig(1, 0.5, true),
		},
		{ // test not enough distinct IPs
			requestCount:   7,
			preferences:    testNodeSelectionConfig(0, 0, true),
			shouldFailWith: &overlay.ErrNotEnoughNodes,
		},
		{ // test distinct flag false allows duplicates
			duplicateCount: 10,
			requestCount:   5,
			preferences:    testNodeSelectionConfig(0, 0.5, false),
		},
	}

	for _, tt := range tests {
		response, err := service.FindStorageNodesWithPreferences(ctx, overlay.FindStorageNodesRequest{
			FreeBandwidth:  0,
			FreeDisk:       0,
			RequestedCount: tt.requestCount,
		}, &tt.preferences)
		if tt.shouldFailWith != nil {
			assert.Error(t, err)
			assert.True(t, tt.shouldFailWith.Has(err))
			continue
		} else {
			require.NoError(t, err)
		}

		// assert all IPs are unique
		if tt.preferences.DistinctIP {
			ips := make(map[string]bool)
			for _, n := range response {
				assert.False(t, ips[n.LastIp])
				ips[n.LastIp] = true
			}
		}

		assert.Equal(t, tt.requestCount, len(response))
	}
}

func TestAddrtoNetwork_Conversion(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ip := "8.8.8.8:28967"
	network, err := overlay.GetNetwork(ctx, ip)
	require.Equal(t, "8.8.8.0", network)
	require.NoError(t, err)

	ipv6 := "[fc00::1:200]:28967"
	network, err = overlay.GetNetwork(ctx, ipv6)
	require.Equal(t, "fc00::", network)
	require.NoError(t, err)
}
