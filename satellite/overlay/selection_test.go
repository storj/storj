// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
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
				config.Overlay.NodeSelectionCache.Staleness = lowStaleness
				config.Overlay.NodeCheckInWaitPeriod = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		saOverlay := planet.Satellites[0].Overlay
		nodeConfig := planet.Satellites[0].Config.Overlay.Node

		node0 := planet.StorageNodes[0]
		node0.Contact.Chore.Pause(ctx)
		nodeInfo := node0.Contact.Service.Local()
		ident := node0.Identity
		peer := rpcpeer.Peer{
			Addr: &net.TCPAddr{
				IP:   net.ParseIP(nodeInfo.Address),
				Port: 5,
			},
			State: tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
			},
		}
		peerCtx := rpcpeer.NewContext(ctx, &peer)

		// report disk space less than minimum
		_, err := planet.Satellites[0].Contact.Endpoint.CheckIn(peerCtx, &pb.CheckInRequest{
			Address: nodeInfo.Address,
			Version: &nodeInfo.Version,
			Capacity: &pb.NodeCapacity{
				FreeDisk: 9 * memory.MB.Int64(),
			},
			Operator: &nodeInfo.Operator,
		})
		require.NoError(t, err)

		req := overlay.FindStorageNodesRequest{
			RequestedCount: 2,
		}

		// request 2 nodes, expect failure from not enough nodes
		n1, err := saOverlay.Service.FindStorageNodesForUpload(ctx, req)
		require.Error(t, err)
		require.True(t, overlay.ErrNotEnoughNodes.Has(err))
		n2, err := saOverlay.Service.UploadSelectionCache.GetNodes(ctx, req)
		require.Error(t, err)
		require.True(t, overlay.ErrNotEnoughNodes.Has(err))
		require.Equal(t, len(n2), len(n1))
		n3, err := saOverlay.Service.FindStorageNodesWithPreferences(ctx, req, &nodeConfig)
		require.Error(t, err)
		require.Equal(t, len(n3), len(n1))

		// report disk space greater than minimum
		_, err = planet.Satellites[0].Contact.Endpoint.CheckIn(peerCtx, &pb.CheckInRequest{
			Address: nodeInfo.Address,
			Version: &nodeInfo.Version,
			Capacity: &pb.NodeCapacity{
				FreeDisk: 11 * memory.MB.Int64(),
			},
			Operator: &nodeInfo.Operator,
		})
		require.NoError(t, err)

		// request 2 nodes, expect success
		n1, err = planet.Satellites[0].Overlay.Service.FindStorageNodesForUpload(ctx, req)
		require.NoError(t, err)
		require.Equal(t, 2, len(n1))
		n2, err = saOverlay.Service.FindStorageNodesWithPreferences(ctx, req, &nodeConfig)
		require.NoError(t, err)
		require.Equal(t, len(n1), len(n2))
		n3, err = saOverlay.Service.UploadSelectionCache.GetNodes(ctx, req)
		require.NoError(t, err)
		require.Equal(t, len(n1), len(n3))
	})
}

func TestOnlineOffline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		service := satellite.Overlay.Service

		online, offline, err := service.KnownReliable(ctx, []storj.NodeID{
			planet.StorageNodes[0].ID(),
		})
		require.NoError(t, err)
		require.Empty(t, offline)
		require.Len(t, online, 1)

		online, offline, err = service.KnownReliable(ctx, []storj.NodeID{
			planet.StorageNodes[0].ID(),
			planet.StorageNodes[1].ID(),
			planet.StorageNodes[2].ID(),
		})
		require.NoError(t, err)
		require.Empty(t, offline)
		require.Len(t, online, 3)

		unreliableNodeID := storj.NodeID{1, 2, 3, 4}
		online, offline, err = service.KnownReliable(ctx, []storj.NodeID{
			planet.StorageNodes[0].ID(),
			unreliableNodeID,
			planet.StorageNodes[2].ID(),
		})
		require.NoError(t, err)
		require.Empty(t, offline)
		require.Len(t, online, 2)

		require.False(t, slices.ContainsFunc(online, func(node overlay.SelectedNode) bool {
			return node.ID == unreliableNodeID
		}))
		require.False(t, slices.ContainsFunc(offline, func(node overlay.SelectedNode) bool {
			return node.ID == unreliableNodeID
		}))
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
				config.Reputation.InitialAlpha = 1
				config.Reputation.AuditLambda = 1
				config.Reputation.UnknownAuditLambda = 1
				config.Reputation.AuditWeight = 1
				config.Reputation.AuditDQ = 0.5
				config.Reputation.UnknownAuditDQ = 0.5
				config.Reputation.AuditCount = 1
				config.Reputation.AuditHistory = testAuditHistoryConfig()
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// pause chores that might update node data
		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()
		for _, node := range planet.StorageNodes {
			node.Contact.Chore.Pause(ctx)
		}

		service := satellite.Overlay.Service
		repService := satellite.Reputation.Service

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
			err := repService.ApplyAudit(ctx, node.ID(), overlay.ReputationStatus{}, reputation.AuditSuccess)
			require.NoError(t, err)
		}
		err := repService.TestFlushAllNodeInfo(ctx)
		require.NoError(t, err)

		t.Run("request 5, where 1 new", func(t *testing.T) {
			requestedCount, newCount := 5, 1
			newNodeFraction := float64(newCount) / float64(requestedCount)
			preferences := testNodeSelectionConfig(newNodeFraction)
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
			preferences := testNodeSelectionConfig(newNodeFraction)
			req := overlay.FindStorageNodesRequest{
				RequestedCount: requestedCount,
			}
			nodes, err := service.FindStorageNodesWithPreferences(ctx, req, &preferences)
			require.NoError(t, err)
			require.Len(t, nodes, requestedCount)
			require.Equal(t, 0, countReputable(nodes))

			n2, err := service.UploadSelectionCache.GetNodes(ctx, req)
			require.NoError(t, err)
			require.Equal(t, requestedCount, len(n2))
		})

		// update all of them to be reputable
		for i := 5; i < 10; i++ {
			node := planet.StorageNodes[i]
			reputable[node.ID()] = true
			err := repService.ApplyAudit(ctx, node.ID(), overlay.ReputationStatus{}, reputation.AuditSuccess)
			require.NoError(t, err)
		}

		t.Run("no new nodes", func(t *testing.T) {
			requestedCount, newCount := 5, 1.0
			newNodeFraction := newCount / float64(requestedCount)
			preferences := testNodeSelectionConfig(newNodeFraction)
			satellite.Config.Overlay.Node = testNodeSelectionConfig(newNodeFraction)

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
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.AuditHistory = testAuditHistoryConfig()
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		service := satellite.Overlay.Service
		errNotEnoughNodes := &overlay.ErrNotEnoughNodes
		tests := []struct {
			description     string
			requestCount    int
			newNodeFraction float64
			reputableNodes  int
			expectedCount   int
			shouldFailWith  *errs.Class
			exclude         func() (excludedNodes []storj.NodeID)
		}{
			{
				description:     "all reputable nodes, only reputable nodes requested",
				requestCount:    6,
				newNodeFraction: 0,
				reputableNodes:  6,
				expectedCount:   6,
			},
			{
				description:     "all reputable nodes, up to 100% new nodes requested",
				requestCount:    5,
				newNodeFraction: 1,
				reputableNodes:  6,
				expectedCount:   5,
			},
			{
				description:     "3 reputable and 3 new nodes, 6 reputable nodes requested, not enough reputable nodes",
				requestCount:    6,
				newNodeFraction: 0,
				reputableNodes:  3,
				expectedCount:   3,
				shouldFailWith:  errNotEnoughNodes,
			},
			{
				description:     "50-50 reputable and new nodes, reputable and new nodes requested, not enough reputable nodes",
				requestCount:    5,
				newNodeFraction: 0.2,
				reputableNodes:  3,
				expectedCount:   4,
				shouldFailWith:  errNotEnoughNodes,
			},
			{
				description:     "all new nodes except one, reputable and new nodes requested (happy path)",
				requestCount:    2,
				newNodeFraction: 0.5,
				reputableNodes:  1,
				expectedCount:   2,
			},
			{
				description:     "all new nodes except one, reputable and new nodes requested (not happy path)",
				requestCount:    4,
				newNodeFraction: 0.5,
				reputableNodes:  1,
				expectedCount:   3,
				shouldFailWith:  errNotEnoughNodes,
			},
			{
				description:     "all new nodes, reputable and new nodes requested",
				requestCount:    6,
				newNodeFraction: 1,
				reputableNodes:  0,
				expectedCount:   6,
			},
			{
				description:     "excluded node ids",
				requestCount:    6,
				newNodeFraction: 0,
				reputableNodes:  6,
				expectedCount:   1,
				shouldFailWith:  errNotEnoughNodes,
				exclude: func() (excludedNodes []storj.NodeID) {
					for _, storageNode := range planet.StorageNodes[:5] {
						excludedNodes = append(excludedNodes, storageNode.ID())
					}
					return excludedNodes
				},
			},
		}

		for _, tt := range tests {
			t.Log(tt.description)
			var excludedNodes []storj.NodeID
			if tt.exclude != nil {
				excludedNodes = tt.exclude()
			}
			for i, node := range planet.StorageNodes {
				if i < tt.reputableNodes {
					_, err := satellite.Overlay.Service.TestVetNode(ctx, node.ID())
					require.NoError(t, err)
				} else {
					err := satellite.Overlay.Service.TestUnvetNode(ctx, node.ID())
					require.NoError(t, err)
				}
			}
			config := testNodeSelectionConfig(tt.newNodeFraction)
			response, err := service.FindStorageNodesWithPreferences(ctx, overlay.FindStorageNodesRequest{RequestedCount: tt.requestCount, ExcludedIDs: excludedNodes}, &config)
			if tt.shouldFailWith != nil {
				require.Error(t, err)
				assert.True(t, tt.shouldFailWith.Has(err))
			} else {
				require.NoError(t, err)
			}
			if len(excludedNodes) > 0 {
				for _, n := range response {
					for _, m := range excludedNodes {
						require.NotEqual(t, n.ID, m)
					}
				}
			}
			require.Equal(t, tt.expectedCount, len(response))
		}
	})
}

func TestNodeSelectionGracefulExit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.MinimumDiskSpace = 10 * memory.MB
				config.Reputation.InitialAlpha = 1
				config.Reputation.AuditLambda = 1
				config.Reputation.UnknownAuditLambda = 1
				config.Reputation.AuditWeight = 1
				config.Reputation.AuditDQ = 0.5
				config.Reputation.UnknownAuditDQ = 0.5
				config.Reputation.AuditHistory = testAuditHistoryConfig()
				config.Reputation.AuditCount = 5 // need 5 audits to be vetted
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		exitingNodes := make(map[storj.NodeID]bool)

		// This sets audit counts of 0, 1, 2, 3, ... 9
		// so that we can fine-tune how many nodes are considered new or reputable
		// by modifying the audit count cutoff passed into FindStorageNodesWithPreferences
		// nodes at indices 0, 2, 4, 6, 8 are gracefully exiting
		for i, node := range planet.StorageNodes {
			for k := 0; k < i; k++ {
				err := satellite.Reputation.Service.ApplyAudit(ctx, node.ID(), overlay.ReputationStatus{}, reputation.AuditSuccess)
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

		// There are now 5 new nodes, and 5 reputable (vetted) nodes. 3 of the
		// new nodes are gracefully exiting, and 2 of the reputable nodes.
		type test struct {
			Preferences    overlay.NodeSelectionConfig
			ExcludeCount   int
			RequestCount   int
			ExpectedCount  int
			ShouldFailWith *errs.Class
		}

		for i, tt := range []test{
			{ // reputable and new nodes, happy path
				Preferences:   testNodeSelectionConfig(0.5),
				RequestCount:  5,
				ExpectedCount: 5, // 2 new + 3 vetted
			},
			{ // all reputable nodes, happy path
				Preferences:   testNodeSelectionConfig(0),
				RequestCount:  3,
				ExpectedCount: 3,
			},
			{ // all new nodes, happy path
				Preferences:   testNodeSelectionConfig(1),
				RequestCount:  2,
				ExpectedCount: 2,
			},
			{ // reputable and new nodes, requested too many
				Preferences:    testNodeSelectionConfig(0.5),
				RequestCount:   10,
				ExpectedCount:  5, // 2 new + 3 vetted
				ShouldFailWith: &overlay.ErrNotEnoughNodes,
			},
			{ // all reputable nodes, requested too many
				Preferences:    testNodeSelectionConfig(0),
				RequestCount:   10,
				ExpectedCount:  3,
				ShouldFailWith: &overlay.ErrNotEnoughNodes,
			},
			{ // all new nodes, requested too many
				Preferences:    testNodeSelectionConfig(1),
				RequestCount:   10,
				ExpectedCount:  2,
				ShouldFailWith: &overlay.ErrNotEnoughNodes,
			},
		} {
			t.Logf("#%2d. %+v", i, tt)

			response, err := satellite.Overlay.Service.FindStorageNodesWithPreferences(ctx,
				overlay.FindStorageNodesRequest{
					RequestedCount:     tt.RequestCount,
					AsOfSystemInterval: -time.Microsecond,
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
			RequestedCount: 2,
			ExcludedIDs:    excludedNodes,
		}
		nodes, err := satellite.Overlay.Service.FindStorageNodesForUpload(ctx, req)
		require.NoError(t, err)
		require.Len(t, nodes, 2)
		require.NotEqual(t, nodes[0].LastIPPort, nodes[1].LastIPPort)
		require.NotEqual(t, nodes[0].LastIPPort, excludedNodeAddr)
		require.NotEqual(t, nodes[1].LastIPPort, excludedNodeAddr)
		n2, err := satellite.Overlay.Service.UploadSelectionCache.GetNodes(ctx, req)
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
			RequestedCount: 4,
			ExcludedIDs:    excludedNodes,
		}
		n, err := satellite.Overlay.Service.FindStorageNodesForUpload(ctx, req)
		require.Error(t, err)
		n1, err := satellite.Overlay.Service.FindStorageNodesWithPreferences(ctx, req, &satellite.Config.Overlay.Node)
		require.Error(t, err)
		require.Equal(t, len(n), len(n1))
		n2, err = satellite.Overlay.Service.UploadSelectionCache.GetNodes(ctx, req)
		require.Error(t, err)
		require.Equal(t, len(n1), len(n2))
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
			RequestedCount: 2,
			ExcludedIDs:    excludedNodes,
		}
		nodes, err := satellite.Overlay.Service.FindStorageNodesForUpload(ctx, req)
		require.NoError(t, err)
		require.Len(t, nodes, 2)
		require.NotEqual(t, nodes[0].LastIPPort, nodes[1].LastIPPort)
		require.NotEqual(t, nodes[0].LastIPPort, excludedNodeAddr)
		require.NotEqual(t, nodes[1].LastIPPort, excludedNodeAddr)
		n2, err := satellite.Overlay.Service.UploadSelectionCache.GetNodes(ctx, req)
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
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.InitialAlpha = 1
				config.Reputation.AuditLambda = 1
				config.Reputation.UnknownAuditLambda = 1
				config.Reputation.AuditWeight = 1
				config.Reputation.AuditDQ = 0.5
				config.Reputation.UnknownAuditDQ = 0.5
				config.Reputation.AuditHistory = testAuditHistoryConfig()
				config.Reputation.AuditCount = 1
				config.Overlay.Node.DistinctIP = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		// Vets nodes[8] and nodes[9].
		for i := 9; i > 7; i-- {
			err := satellite.Reputation.Service.ApplyAudit(ctx, planet.StorageNodes[i].ID(), overlay.ReputationStatus{}, reputation.AuditSuccess)
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
			UniqueIPCount: 3, // creates 3 additional unique ip addresses, totaling to 4 IPs
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.UpdateStatsBatchSize = 1
				config.Reputation.InitialAlpha = 1
				config.Reputation.AuditLambda = 1
				config.Reputation.UnknownAuditLambda = 1
				config.Reputation.AuditWeight = 1
				config.Reputation.AuditDQ = 0.5
				config.Reputation.UnknownAuditDQ = 0.5
				config.Reputation.AuditHistory = testAuditHistoryConfig()
				config.Reputation.AuditCount = 1
				config.Overlay.Node.DistinctIP = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		// Vets nodes[8] and nodes[9].
		for i := 9; i > 7; i-- {
			err := satellite.Reputation.Service.ApplyAudit(ctx, planet.StorageNodes[i].ID(), overlay.ReputationStatus{}, reputation.AuditSuccess)
			assert.NoError(t, err)
		}
		testDistinctIPs(t, ctx, planet)
	})
}

func testDistinctIPs(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
	satellite := planet.Satellites[0]
	service := satellite.Overlay.Service

	tests := []struct {
		requestCount   int
		preferences    overlay.NodeSelectionConfig
		shouldFailWith *errs.Class
	}{
		{ // test only distinct IPs with half new nodes
			// expect 2 new and 2 vetted
			requestCount: 4,
			preferences:  testNodeSelectionConfig(0.5),
		},
		{ // test not enough distinct IPs
			requestCount:   5, // expect 3 new, 2 old but fails because only 4 distinct IPs, not 5
			preferences:    testNodeSelectionConfig(0.6),
			shouldFailWith: &overlay.ErrNotEnoughNodes,
		},
	}

	for _, tt := range tests {
		response, err := service.FindStorageNodesWithPreferences(ctx,
			overlay.FindStorageNodesRequest{
				RequestedCount:     tt.requestCount,
				AsOfSystemInterval: -time.Microsecond,
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

	runTest := func(t *testing.T, ipAddr, port string, distinctIPEnabled bool, ipv4Mask, ipv6Mask int, expectedNetwork string) {
		t.Run(fmt.Sprintf("%s-%s-%v-%d-%d", ipAddr, port, distinctIPEnabled, ipv4Mask, ipv6Mask), func(t *testing.T) {
			ipAndPort := net.JoinHostPort(ipAddr, port)
			config := overlay.NodeSelectionConfig{
				DistinctIP:        distinctIPEnabled,
				NetworkPrefixIPv4: ipv4Mask,
				NetworkPrefixIPv6: ipv6Mask,
			}
			resolvedIP, resolvedPort, network, err := overlay.ResolveIPAndNetwork(ctx, ipAndPort, config, overlay.MaskOffLastNet)
			require.NoError(t, err)
			assert.Equal(t, expectedNetwork, network)
			assert.Equal(t, ipAddr, resolvedIP.String())
			assert.Equal(t, port, resolvedPort)
		})
	}

	runTest(t, "8.8.255.8", "28967", true, 17, 128, "8.8.128.0")
	runTest(t, "8.8.255.8", "28967", false, 0, 0, "8.8.255.8:28967")

	runTest(t, "fc00::1:200", "28967", true, 0, 64, "fc00::")
	runTest(t, "fc00::1:200", "28967", true, 0, 128-16, "fc00::1:0")
	runTest(t, "fc00::1:200", "28967", false, 0, 0, "[fc00::1:200]:28967")
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
		n2, err := saOverlay.Service.UploadSelectionCache.GetNodes(ctx, req)
		require.NoError(t, err)
		require.Equal(t, len(n2), len(n1))
		n3, err := saOverlay.Service.FindStorageNodesWithPreferences(ctx, req, &nodeConfig)
		require.NoError(t, err)
		require.Equal(t, len(n3), len(n2))
	})
}
