// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
)

func TestMinimumDiskSpace(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Test does not work with macOS")
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			UniqueIPCount: 2,
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.MinimumDiskSpace = 10 * memory.MB
				config.Overlay.NodeSelectionCache.Staleness = -time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		saOverlay := planet.Satellites[0].Overlay
		nodeConfig := planet.Satellites[0].Config.Overlay.Node

		node0 := planet.StorageNodes[0]
		node0.Contact.Chore.Pause(ctx)
		nodeDossier := node0.Local()
		ident := node0.Identity
		peer := rpcpeer.Peer{
			Addr: &net.TCPAddr{
				IP:   net.ParseIP(nodeDossier.Address.GetAddress()),
				Port: 5,
			},
			State: tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
			},
		}
		peerCtx := rpcpeer.NewContext(ctx, &peer)

		// report disk space less than minimum
		_, err := planet.Satellites[0].Contact.Endpoint.CheckIn(peerCtx, &pb.CheckInRequest{
			Address: nodeDossier.Address.GetAddress(),
			Version: &nodeDossier.Version,
			Capacity: &pb.NodeCapacity{
				FreeDisk: 9 * memory.MB.Int64(),
			},
			Operator: &nodeDossier.Operator,
		})
		require.NoError(t, err)

		req := overlay.FindStorageNodesRequest{
			MinimumRequiredNodes: 2,
			RequestedCount:       2,
		}

		// request 2 nodes, expect failure from not enough nodes
		n1, err := saOverlay.Service.FindStorageNodesForUpload(ctx, req)
		require.Error(t, err)
		require.True(t, overlay.ErrNotEnoughNodes.Has(err))
		n2, err := saOverlay.Service.SelectionCache.GetNodes(ctx, req)
		require.Error(t, err)
		require.True(t, overlay.ErrNotEnoughNodes.Has(err))
		require.Equal(t, len(n2), len(n1))
		n3, err := saOverlay.Service.FindStorageNodesWithPreferences(ctx, req, &nodeConfig)
		require.Error(t, err)
		require.Equal(t, len(n3), len(n1))

		// report disk space greater than minimum
		_, err = planet.Satellites[0].Contact.Endpoint.CheckIn(peerCtx, &pb.CheckInRequest{
			Address: nodeDossier.Address.GetAddress(),
			Version: &nodeDossier.Version,
			Capacity: &pb.NodeCapacity{
				FreeDisk: 11 * memory.MB.Int64(),
			},
			Operator: &nodeDossier.Operator,
		})
		require.NoError(t, err)

		// request 2 nodes, expect success
		n1, err = planet.Satellites[0].Overlay.Service.FindStorageNodesForUpload(ctx, req)
		require.NoError(t, err)
		require.Equal(t, 2, len(n1))
		n2, err = saOverlay.Service.FindStorageNodesWithPreferences(ctx, req, &nodeConfig)
		require.NoError(t, err)
		require.Equal(t, len(n1), len(n2))
		n3, err = saOverlay.Service.SelectionCache.GetNodes(ctx, req)
		require.NoError(t, err)
		require.Equal(t, len(n1), len(n3))
	})
}

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

func TestEnsureMinimumRequested(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Test does not work with macOS")
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			UniqueIPCount: 5,
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.MinimumDiskSpace = 10 * memory.MB
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// pause chores that might update node data
		satellite.Audit.Chore.Loop.Pause()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.DowntimeTracking.DetectionChore.Loop.Pause()
		satellite.DowntimeTracking.EstimationChore.Loop.Pause()
		for _, node := range planet.StorageNodes {
			node.Contact.Chore.Pause(ctx)
		}

		service := satellite.Overlay.Service

		reputable := map[storj.NodeID]bool{}

		countReputable := func(selected []*overlay.SelectedNode) (count int) {
			for _, n := range selected {
				if reputable[n.ID] {
					count++
				}
			}
			return count
		}

		// update half of nodes to be reputable
		for i := 0; i < 5; i++ {
			node := planet.StorageNodes[i]
			reputable[node.ID()] = true
			_, err := satellite.DB.OverlayCache().UpdateStats(ctx, &overlay.UpdateRequest{
				NodeID:       node.ID(),
				IsUp:         true,
				AuditOutcome: overlay.AuditSuccess,
				AuditLambda:  1, AuditWeight: 1, AuditDQ: 0.5,
			})
			require.NoError(t, err)
		}

		t.Run("request 5, where 1 new", func(t *testing.T) {
			requestedCount, newCount := 5, 1
			newNodeFraction := float64(newCount) / float64(requestedCount)
			preferences := testNodeSelectionConfig(1, newNodeFraction, false)
			req := overlay.FindStorageNodesRequest{
				RequestedCount: requestedCount,
			}
			nodes, err := service.FindStorageNodesWithPreferences(ctx, req, &preferences)
			require.NoError(t, err)
			require.Len(t, nodes, requestedCount)
			require.Equal(t, requestedCount-newCount, countReputable(nodes))
		})

		t.Run("request 5, all new", func(t *testing.T) {
			requestedCount, newCount := 5, 5
			newNodeFraction := float64(newCount) / float64(requestedCount)
			preferences := testNodeSelectionConfig(1, newNodeFraction, false)
			req := overlay.FindStorageNodesRequest{
				RequestedCount: requestedCount,
			}
			nodes, err := service.FindStorageNodesWithPreferences(ctx, req, &preferences)
			require.NoError(t, err)
			require.Len(t, nodes, requestedCount)
			require.Equal(t, 0, countReputable(nodes))

			n2, err := service.SelectionCache.GetNodes(ctx, req)
			require.NoError(t, err)
			require.Equal(t, requestedCount, len(n2))
		})

		// update all of them to be reputable
		for i := 5; i < 10; i++ {
			node := planet.StorageNodes[i]
			reputable[node.ID()] = true
			_, err := satellite.DB.OverlayCache().UpdateStats(ctx, &overlay.UpdateRequest{
				NodeID:       node.ID(),
				IsUp:         true,
				AuditOutcome: overlay.AuditSuccess,
				AuditLambda:  1, AuditWeight: 1, AuditDQ: 0.5,
			})
			require.NoError(t, err)
		}

		t.Run("no new nodes", func(t *testing.T) {
			requestedCount, newCount := 5, 1.0
			newNodeFraction := newCount / float64(requestedCount)
			preferences := testNodeSelectionConfig(1, newNodeFraction, false)
			satellite.Config.Overlay.Node = testNodeSelectionConfig(1, newNodeFraction, false)

			nodes, err := service.FindStorageNodesWithPreferences(ctx, overlay.FindStorageNodesRequest{
				RequestedCount: requestedCount,
			}, &preferences)
			require.NoError(t, err)
			require.Len(t, nodes, requestedCount)
			// all of them should be reputable because there are no new nodes
			require.Equal(t, 5, countReputable(nodes))
		})
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
					AuditOutcome: overlay.AuditSuccess,
					AuditLambda:  1, AuditWeight: 1, AuditDQ: 0.5,
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
					AuditOutcome: overlay.AuditSuccess,
					AuditLambda:  1, AuditWeight: 1, AuditDQ: 0.5,
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
		n := storageNode.Local()
		d := overlay.NodeCheckInInfo{
			NodeID:     storageNode.ID(),
			Address:    n.Address,
			LastIPPort: storageNode.Addr(),
			LastNet:    n.LastNet,
			Version:    &n.Version,
		}
		err := satellite.Overlay.DB.UpdateCheckIn(ctx, d, time.Now().UTC(), satellite.Config.Overlay.Node)
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
			RequestedCount: tt.RequestCount,
			ExcludedIDs:    excludedNodes,
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
					AuditOutcome: overlay.AuditSuccess,
					AuditLambda:  1, AuditWeight: 1, AuditDQ: 0.5,
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
				assert.False(t, exitingNodes[node.ID])
			}
		}
	})
}

func TestFindStorageNodesDistinctNetworks(t *testing.T) {
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
		excludedNodeAddr = res.LastIPPort

		req := overlay.FindStorageNodesRequest{
			MinimumRequiredNodes: 2,
			RequestedCount:       2,
			ExcludedIDs:          excludedNodes,
		}
		nodes, err := satellite.Overlay.Service.FindStorageNodesForUpload(ctx, req)
		require.NoError(t, err)
		require.Len(t, nodes, 2)
		require.NotEqual(t, nodes[0].LastIPPort, nodes[1].LastIPPort)
		require.NotEqual(t, nodes[0].LastIPPort, excludedNodeAddr)
		require.NotEqual(t, nodes[1].LastIPPort, excludedNodeAddr)
		n2, err := satellite.Overlay.Service.SelectionCache.GetNodes(ctx, req)
		require.NoError(t, err)
		require.Len(t, n2, 2)
		require.NotEqual(t, n2[0].LastIPPort, n2[1].LastIPPort)
		require.NotEqual(t, n2[0].LastIPPort, excludedNodeAddr)
		require.NotEqual(t, n2[1].LastIPPort, excludedNodeAddr)
		n3, err := satellite.Overlay.Service.FindStorageNodesWithPreferences(ctx, req, &satellite.Config.Overlay.Node)
		require.NoError(t, err)
		require.Len(t, n3, 2)
		require.NotEqual(t, n3[0].LastIPPort, n3[1].LastIPPort)
		require.NotEqual(t, n3[0].LastIPPort, excludedNodeAddr)
		require.NotEqual(t, n3[1].LastIPPort, excludedNodeAddr)

		req = overlay.FindStorageNodesRequest{
			MinimumRequiredNodes: 4,
			RequestedCount:       4,
			ExcludedIDs:          excludedNodes,
		}
		n, err := satellite.Overlay.Service.FindStorageNodesForUpload(ctx, req)
		require.Error(t, err)
		n1, err := satellite.Overlay.Service.FindStorageNodesWithPreferences(ctx, req, &satellite.Config.Overlay.Node)
		require.Error(t, err)
		require.Equal(t, len(n), len(n1))
		n2, err = satellite.Overlay.Service.SelectionCache.GetNodes(ctx, req)
		require.Error(t, err)
		// GetNodes returns 1 more node than FindStorageNodesWithPreferences because of the way the queries are...
		// FindStorageNodesWithPreferences gets the IPs for the excludedNodeIDs and excludes all those IPs from the selection
		// (which results in filtering out any node on the same network as a excludedNodeID),
		// but the selection cache only filters IPs at time of selection which makes it so that it can include a node that shares a network
		// with an exclueded ID
		require.Equal(t, len(n1)+1, len(n2))
	})
}

func TestSelectNewStorageNodesExcludedIPs(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Test does not work with macOS")
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			// will create 2 storage nodes with same IP; 2 will have unique
			UniqueIPCount: 2,
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.DistinctIP = true
				config.Overlay.Node.NewNodeFraction = 1
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
			if addrCounts[addrNoPort] > 0 {
				excludedNodes = append(excludedNodes, node.ID())
				break
			}
			addrCounts[addrNoPort]++
		}
		require.Len(t, excludedNodes, 1)
		res, err := satellite.Overlay.Service.Get(ctx, excludedNodes[0])
		require.NoError(t, err)
		excludedNodeAddr = res.LastIPPort

		req := overlay.FindStorageNodesRequest{
			MinimumRequiredNodes: 2,
			RequestedCount:       2,
			ExcludedIDs:          excludedNodes,
		}
		nodes, err := satellite.Overlay.Service.FindStorageNodesForUpload(ctx, req)
		require.NoError(t, err)
		require.Len(t, nodes, 2)
		require.NotEqual(t, nodes[0].LastIPPort, nodes[1].LastIPPort)
		require.NotEqual(t, nodes[0].LastIPPort, excludedNodeAddr)
		require.NotEqual(t, nodes[1].LastIPPort, excludedNodeAddr)
		n2, err := satellite.Overlay.Service.SelectionCache.GetNodes(ctx, req)
		require.NoError(t, err)
		require.Len(t, n2, 2)
		require.NotEqual(t, n2[0].LastIPPort, n2[1].LastIPPort)
		require.NotEqual(t, n2[0].LastIPPort, excludedNodeAddr)
		require.NotEqual(t, n2[1].LastIPPort, excludedNodeAddr)
		n3, err := satellite.Overlay.Service.FindStorageNodesWithPreferences(ctx, req, &satellite.Config.Overlay.Node)
		require.NoError(t, err)
		require.Len(t, n3, 2)
		require.NotEqual(t, n3[0].LastIPPort, n3[1].LastIPPort)
		require.NotEqual(t, n3[0].LastIPPort, excludedNodeAddr)
		require.NotEqual(t, n3[1].LastIPPort, excludedNodeAddr)
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
				AuditOutcome: overlay.AuditSuccess,
				AuditLambda:  1,
				AuditWeight:  1,
				AuditDQ:      0.5,
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
				AuditOutcome: overlay.AuditSuccess,
				AuditLambda:  1,
				AuditWeight:  1,
				AuditDQ:      0.5,
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
				assert.False(t, ips[n.LastIPPort])
				ips[n.LastIPPort] = true
			}
		}

		assert.Equal(t, tt.requestCount, len(response))
	}
}

func TestAddrtoNetwork_Conversion(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ip := "8.8.8.8:28967"
	resolvedIPPort, network, err := overlay.ResolveIPAndNetwork(ctx, ip)
	require.Equal(t, "8.8.8.0", network)
	require.Equal(t, ip, resolvedIPPort)
	require.NoError(t, err)

	ipv6 := "[fc00::1:200]:28967"
	resolvedIPPort, network, err = overlay.ResolveIPAndNetwork(ctx, ipv6)
	require.Equal(t, "fc00::", network)
	require.Equal(t, ipv6, resolvedIPPort)
	require.NoError(t, err)
}

func TestCacheSelectionVsDBSelection(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Test does not work with macOS")
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			UniqueIPCount: 5,
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.StorageNodes[0].Storage2.Monitor.Loop.Pause()
		saOverlay := planet.Satellites[0].Overlay
		nodeConfig := planet.Satellites[0].Config.Overlay.Node

		req := overlay.FindStorageNodesRequest{RequestedCount: 5}
		n1, err := saOverlay.Service.FindStorageNodesForUpload(ctx, req)
		require.NoError(t, err)
		n2, err := saOverlay.Service.SelectionCache.GetNodes(ctx, req)
		require.NoError(t, err)
		require.Equal(t, len(n2), len(n1))
		n3, err := saOverlay.Service.FindStorageNodesWithPreferences(ctx, req, &nodeConfig)
		require.NoError(t, err)
		require.Equal(t, len(n3), len(n2))
	})
}
