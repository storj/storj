// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/rand"
	"net"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity/testidentity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/nodeselection"
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
				config.Overlay.NodeSelectionCache.Staleness = lowStaleness
				config.Overlay.NodeCheckInWaitPeriod = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		saOverlay := planet.Satellites[0].Overlay

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
		n3, err := saOverlay.Service.UploadSelectionCache.GetNodes(ctx, req)
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

		selectedNodes, err := service.GetParticipatingNodes(ctx, []storj.NodeID{
			planet.StorageNodes[0].ID(),
		})
		require.NoError(t, err)
		require.Len(t, selectedNodes, 1)
		require.True(t, selectedNodes[0].Online)

		selectedNodes, err = service.GetParticipatingNodes(ctx, []storj.NodeID{
			planet.StorageNodes[0].ID(),
			planet.StorageNodes[1].ID(),
			planet.StorageNodes[2].ID(),
		})
		require.NoError(t, err)
		require.Len(t, selectedNodes, 3)
		for i := 0; i < 3; i++ {
			require.True(t, selectedNodes[i].Online, i)
			require.Equal(t, planet.StorageNodes[i].ID(), selectedNodes[i].ID, i)
		}

		unreliableNodeID := storj.NodeID{1, 2, 3, 4}
		selectedNodes, err = service.GetParticipatingNodes(ctx, []storj.NodeID{
			planet.StorageNodes[0].ID(),
			unreliableNodeID,
			planet.StorageNodes[2].ID(),
		})
		require.NoError(t, err)
		require.Len(t, selectedNodes, 3)
		require.True(t, selectedNodes[0].Online)
		require.False(t, selectedNodes[1].Online)
		require.True(t, selectedNodes[2].Online)
		require.Equal(t, planet.StorageNodes[0].ID(), selectedNodes[0].ID)
		require.Equal(t, storj.NodeID{}, selectedNodes[1].ID)
		require.Equal(t, planet.StorageNodes[2].ID(), selectedNodes[2].ID)
	})
}

var defaultNodes = func(i int, node *nodeselection.SelectedNode) {}

func overlayDefaultConfig(newNodeFraction float64) overlay.Config {
	return overlay.Config{
		Node: overlay.NodeSelectionConfig{
			NewNodeFraction: newNodeFraction,
		},
		NodeSelectionCache: overlay.UploadSelectionCacheConfig{
			Staleness: 10 * time.Hour,
		},
	}
}

func TestEnsureMinimumRequested(t *testing.T) {
	ctx := testcontext.New(t)

	t.Run("request 5, where 1 new", func(t *testing.T) {
		t.Parallel()
		requestedCount, newCount := 5, 1
		newNodeFraction := float64(newCount) / float64(requestedCount)

		service, db, cleanup := runServiceWithDB(ctx, zaptest.NewLogger(t), 5, 5, overlayDefaultConfig(newNodeFraction), defaultNodes)
		defer cleanup()

		req := overlay.FindStorageNodesRequest{
			RequestedCount: requestedCount,
		}
		nodes, err := service.FindStorageNodesForUpload(ctx, req)
		require.NoError(t, err)
		require.Len(t, nodes, requestedCount)
		require.Equal(t, requestedCount-newCount, countCommon(db.Reputable, nodes))
	})

	t.Run("request 5, all new", func(t *testing.T) {
		t.Parallel()
		requestedCount, newCount := 5, 5
		newNodeFraction := float64(newCount) / float64(requestedCount)

		service, db, cleanup := runServiceWithDB(ctx, zaptest.NewLogger(t), 5, 5, overlayDefaultConfig(newNodeFraction), defaultNodes)
		defer cleanup()

		req := overlay.FindStorageNodesRequest{
			RequestedCount: requestedCount,
		}
		nodes, err := service.FindStorageNodesForUpload(ctx, req)
		require.NoError(t, err)
		require.Len(t, nodes, requestedCount)
		require.Equal(t, 3, countCommon(db.Reputable, nodes))

		n2, err := service.UploadSelectionCache.GetNodes(ctx, req)
		require.NoError(t, err)
		require.Equal(t, requestedCount, len(n2))
	})

	t.Run("no new nodes", func(t *testing.T) {
		t.Parallel()
		requestedCount, newCount := 5, 1.0
		newNodeFraction := newCount / float64(requestedCount)

		service, db, cleanup := runServiceWithDB(ctx, zaptest.NewLogger(t), 10, 0, overlayDefaultConfig(newNodeFraction), defaultNodes)
		defer cleanup()

		nodes, err := service.FindStorageNodesForUpload(ctx, overlay.FindStorageNodesRequest{
			RequestedCount: requestedCount,
		})
		require.NoError(t, err)
		require.Len(t, nodes, requestedCount)
		// all of them should be reputable because there are no new nodes
		require.Equal(t, 5, countCommon(db.Reputable, nodes))
	})
}

func TestNodeSelection(t *testing.T) {
	errNotEnoughNodes := &overlay.ErrNotEnoughNodes
	tests := []struct {
		description     string
		requestCount    int
		newNodeFraction float64
		reputableNodes  int
		expectedCount   int
		shouldFailWith  *errs.Class
		exclude         int
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
			exclude:         5,
			shouldFailWith:  errNotEnoughNodes,
		},
	}

	ctx := testcontext.New(t)

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {

			service, db, cleanup := runServiceWithDB(ctx, zaptest.NewLogger(t), tt.reputableNodes, 6, overlayDefaultConfig(tt.newNodeFraction), defaultNodes)
			defer cleanup()

			var excludedNodes []storj.NodeID
			if tt.exclude > 0 {
				for i := 0; i < tt.exclude; i++ {
					excludedNodes = append(excludedNodes, db.Reputable[i].ID)
				}

			}

			response, err := service.FindStorageNodesForUpload(ctx, overlay.FindStorageNodesRequest{RequestedCount: tt.requestCount, ExcludedIDs: excludedNodes})
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
		})

	}

}

func TestNodeSelectionGracefulExit(t *testing.T) {
	// There are now 5 new nodes, and 5 reputable (vetted) nodes. 3 of the
	// new nodes are gracefully exiting, and 2 of the reputable nodes.
	type test struct {
		NewNodeFraction float64
		ExcludeCount    int
		RequestCount    int
		ExpectedCount   int
		ShouldFailWith  *errs.Class
	}

	for i, tt := range []test{
		{ // reputable and new nodes, happy path
			NewNodeFraction: 0.5,
			RequestCount:    5,
			ExpectedCount:   5, // 2 new + 3 vetted
		},
		{ // all reputable nodes, happy path
			NewNodeFraction: 0,
			RequestCount:    3,
			ExpectedCount:   3,
		},
		{ // all new nodes, happy path
			NewNodeFraction: 1,
			RequestCount:    2,
			ExpectedCount:   2,
		},
		{ // reputable and new nodes, requested too many
			NewNodeFraction: 0.5,
			RequestCount:    10,
			ExpectedCount:   5, // 2 new + 3 vetted
			ShouldFailWith:  &overlay.ErrNotEnoughNodes,
		},
		{ // all reputable nodes, requested too many
			NewNodeFraction: 0,
			RequestCount:    10,
			ExpectedCount:   3,
			ShouldFailWith:  &overlay.ErrNotEnoughNodes,
		},
		{ // all new nodes, requested too many
			NewNodeFraction: 1,
			RequestCount:    10,
			ExpectedCount:   2,
			ShouldFailWith:  &overlay.ErrNotEnoughNodes,
		},
	} {
		t.Run(fmt.Sprintf("#%2d. %+v", i, tt), func(t *testing.T) {
			ctx := testcontext.New(t)
			service, _, cleanup := runServiceWithDB(ctx, zaptest.NewLogger(t), 5, 0, overlayDefaultConfig(tt.NewNodeFraction), defaultNodes)
			defer cleanup()

			response, err := service.FindStorageNodesForGracefulExit(ctx, overlay.FindStorageNodesRequest{
				RequestedCount: tt.RequestCount,
			})

			t.Log(len(response), err)
			if tt.ShouldFailWith != nil {
				assert.Error(t, err)
				assert.True(t, tt.ShouldFailWith.Has(err))
				return
			}
			assert.NoError(t, err)

			assert.Equal(t, tt.ExpectedCount, len(response))

		})

	}

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

		req = overlay.FindStorageNodesRequest{
			RequestedCount: 4,
			ExcludedIDs:    excludedNodes,
		}
		_, err = satellite.Overlay.Service.FindStorageNodesForUpload(ctx, req)
		require.Error(t, err)
		_, err = satellite.Overlay.Service.UploadSelectionCache.GetNodes(ctx, req)
		require.Error(t, err)
	})
}

func TestSelectNewStorageNodesExcludedIPs(t *testing.T) {
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
		n3, err := satellite.Overlay.Service.UploadSelectionCache.GetNodes(ctx, req)
		require.NoError(t, err)
		require.Len(t, n3, 2)
		require.NotEqual(t, n3[0].LastIPPort, n3[1].LastIPPort)
		require.NotEqual(t, n3[0].LastIPPort, excludedNodeAddr)
		require.NotEqual(t, n3[1].LastIPPort, excludedNodeAddr)
	})
}

func TestDistinctIPs(t *testing.T) {
	tests := []struct {
		requestCount    int
		newNodeFraction float64
		shouldFailWith  *errs.Class
	}{
		{ // test only distinct IPs with half new nodes
			// expect 2 new and 2 vetted
			requestCount:    4,
			newNodeFraction: 0.5,
		},
		{ // test not enough distinct IPs
			requestCount:    5, // expect 3 new, 2 old but fails because only 4 distinct IPs, not 5
			newNodeFraction: 0.6,
			shouldFailWith:  &overlay.ErrNotEnoughNodes,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ctx := testcontext.New(t)
			config := overlayDefaultConfig(tt.newNodeFraction)
			config.Node.DistinctIP = true

			service, _, cleanup := runServiceWithDB(ctx, zaptest.NewLogger(t), 8, 8, config, func(i int, node *nodeselection.SelectedNode) {
				if i < 7 {
					node.LastIPPort = fmt.Sprintf("54.0.0.1:%d", rand.Intn(30000)+1000)
					node.LastNet = "54.0.0.0"
				}
			})
			defer cleanup()

			response, err := service.FindStorageNodesForUpload(ctx,
				overlay.FindStorageNodesRequest{
					RequestedCount: tt.requestCount,
				})
			if tt.shouldFailWith != nil {
				assert.Error(t, err)
				assert.True(t, tt.shouldFailWith.Has(err))
				return
			}
			require.NoError(t, err)

			// assert all IPs are unique
			ips := make(map[string]bool)
			for _, n := range response {
				assert.False(t, ips[n.LastIPPort])
				ips[n.LastIPPort] = true
			}

			assert.Equal(t, tt.requestCount, len(response))
		})

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

func countCommon(reference []*nodeselection.SelectedNode, selected []*nodeselection.SelectedNode) (count int) {
	for _, r := range reference {
		for _, n := range selected {
			if r.ID == n.ID {
				count++
			}
		}
	}
	return count
}

func runServiceWithDB(ctx *testcontext.Context, log *zap.Logger, reputable int, new int, config overlay.Config, nodeCustomization func(i int, node *nodeselection.SelectedNode)) (*overlay.Service, *overlay.Mockdb, func()) {
	db := &overlay.Mockdb{}
	for i := 0; i < reputable+new; i++ {
		node := nodeselection.SelectedNode{
			ID:      testidentity.MustPregeneratedIdentity(i, storj.LatestIDVersion()).ID,
			LastNet: fmt.Sprintf("10.9.%d.0", i),
			Address: &pb.NodeAddress{
				Address: fmt.Sprintf("10.9.%d.1:9999", i),
			},
			LastIPPort: fmt.Sprintf("10.9.%d.1:9999", i),
			Vetted:     i < reputable,
		}
		nodeCustomization(i, &node)
		if i >= reputable {
			db.New = append(db.New, &node)
		} else {
			db.Reputable = append(db.Reputable, &node)
		}
	}
	service, _ := overlay.NewService(log, db, nil, nodeselection.TestPlacementDefinitionsWithFraction(config.Node.NewNodeFraction), "", "", config)
	serviceCtx, cancel := context.WithCancel(ctx)
	ctx.Go(func() error {
		return service.Run(serviceCtx)
	})

	return service, db, cancel
}
