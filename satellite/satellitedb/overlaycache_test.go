// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"context"
	"encoding/binary"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/private/version"
	"storj.io/storj/private/teststorj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/nodeselection/uploadselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestGetOfflineNodesForEmail(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		selectionCfg := overlay.NodeSelectionConfig{
			OnlineWindow: 4 * time.Hour,
		}

		offlineID := teststorj.NodeIDFromString("offlineNode")
		onlineID := teststorj.NodeIDFromString("onlineNode")
		disqualifiedID := teststorj.NodeIDFromString("dqNode")
		exitedID := teststorj.NodeIDFromString("exitedNode")
		offlineNoEmailID := teststorj.NodeIDFromString("noEmail")

		checkInInfo := overlay.NodeCheckInInfo{
			IsUp: true,
			Address: &pb.NodeAddress{
				Address: "1.2.3.4",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			Operator: &pb.NodeOperator{
				Email: "offline@storj.test",
			},
		}

		now := time.Now()

		// offline node should be selected
		checkInInfo.NodeID = offlineID
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now.Add(-24*time.Hour), selectionCfg))

		// online node should not be selected
		checkInInfo.NodeID = onlineID
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now, selectionCfg))

		// disqualified node should not be selected
		checkInInfo.NodeID = disqualifiedID
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now.Add(-24*time.Hour), selectionCfg))
		_, err := cache.DisqualifyNode(ctx, disqualifiedID, now, overlay.DisqualificationReasonUnknown)
		require.NoError(t, err)

		// exited node should not be selected
		checkInInfo.NodeID = exitedID
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now.Add(-24*time.Hour), selectionCfg))
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              exitedID,
			ExitInitiatedAt:     now,
			ExitLoopCompletedAt: now,
			ExitFinishedAt:      now,
			ExitSuccess:         true,
		})
		require.NoError(t, err)

		// node with no email should not be selected
		checkInInfo.NodeID = offlineNoEmailID
		checkInInfo.Operator.Email = ""
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, time.Now().Add(-24*time.Hour), selectionCfg))

		nodes, err := cache.GetOfflineNodesForEmail(ctx, selectionCfg.OnlineWindow, 72*time.Hour, 24*time.Hour, 10)
		require.NoError(t, err)
		require.Equal(t, 1, len(nodes))
		require.NotEmpty(t, nodes[offlineID])

		// test cutoff causes node to not be selected
		nodes, err = cache.GetOfflineNodesForEmail(ctx, selectionCfg.OnlineWindow, time.Second, 24*time.Hour, 10)
		require.NoError(t, err)
		require.Empty(t, nodes)
	})
}

func TestUpdateLastOfflineEmail(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		selectionCfg := overlay.NodeSelectionConfig{
			OnlineWindow: 4 * time.Hour,
		}

		nodeID0 := teststorj.NodeIDFromString("testnode0")
		nodeID1 := teststorj.NodeIDFromString("testnode1")

		checkInInfo := overlay.NodeCheckInInfo{
			IsUp: true,
			Address: &pb.NodeAddress{
				Address: "1.2.3.4",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			Operator: &pb.NodeOperator{
				Email: "test@storj.test",
			},
		}

		now := time.Now()
		checkInInfo.NodeID = nodeID0
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now, selectionCfg))
		checkInInfo.NodeID = nodeID1
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now, selectionCfg))
		require.NoError(t, cache.UpdateLastOfflineEmail(ctx, []storj.NodeID{nodeID0, nodeID1}, now))

		node0, err := cache.Get(ctx, nodeID0)
		require.NoError(t, err)
		require.Equal(t, now.Truncate(time.Second), node0.LastOfflineEmail.Truncate(time.Second))

		node1, err := cache.Get(ctx, nodeID1)
		require.NoError(t, err)
		require.Equal(t, now.Truncate(time.Second), node1.LastOfflineEmail.Truncate(time.Second))
	})
}

func TestSetNodeContained(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		nodeID := testrand.NodeID()
		checkInInfo := overlay.NodeCheckInInfo{
			IsUp: true,
			Address: &pb.NodeAddress{
				Address: "1.2.3.4",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			Operator: &pb.NodeOperator{
				Email: "offline@storj.test",
			},
		}

		now := time.Now()

		// offline node should be selected
		checkInInfo.NodeID = nodeID
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now.Add(-24*time.Hour), overlay.NodeSelectionConfig{}))

		cacheInfo, err := cache.Get(ctx, nodeID)
		require.NoError(t, err)
		require.False(t, cacheInfo.Contained)

		err = cache.SetNodeContained(ctx, nodeID, true)
		require.NoError(t, err)

		cacheInfo, err = cache.Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, cacheInfo.Contained)

		err = cache.SetNodeContained(ctx, nodeID, false)
		require.NoError(t, err)

		cacheInfo, err = cache.Get(ctx, nodeID)
		require.NoError(t, err)
		require.False(t, cacheInfo.Contained)
	})
}

func TestUpdateCheckInDirectUpdate(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()
		db.OverlayCache()
		selectionCfg := overlay.NodeSelectionConfig{
			OnlineWindow: 4 * time.Hour,
		}
		nodeID := teststorj.NodeIDFromString("testnode0")
		checkInInfo := overlay.NodeCheckInInfo{
			IsUp: true,
			Address: &pb.NodeAddress{
				Address: "1.2.3.4",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			Operator: &pb.NodeOperator{
				Email: "test@storj.test",
			},
		}
		now := time.Now().UTC()
		checkInInfo.NodeID = nodeID
		semVer, err := version.NewSemVer(checkInInfo.Version.Version)
		require.NoError(t, err)
		// node unknown - should not be updated by updateCheckInDirectUpdate
		updated, err := cache.TestUpdateCheckInDirectUpdate(ctx, checkInInfo, now, semVer, "encodedwalletfeature")
		require.NoError(t, err)
		require.False(t, updated)
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now, selectionCfg))
		updated, err = cache.TestUpdateCheckInDirectUpdate(ctx, checkInInfo, now.Add(6*time.Hour), semVer, "encodedwalletfeature")
		require.NoError(t, err)
		require.True(t, updated)
	})
}

func TestSetAllContainedNodes(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		node1 := testrand.NodeID()
		node2 := testrand.NodeID()
		node3 := testrand.NodeID()

		// put nodes with these IDs in the db
		for _, n := range []storj.NodeID{node1, node2, node3} {
			checkInInfo := overlay.NodeCheckInInfo{
				IsUp:    true,
				Address: &pb.NodeAddress{Address: "1.2.3.4"},
				Version: &pb.NodeVersion{Version: "v0.0.0"},
				NodeID:  n,
			}
			err := cache.UpdateCheckIn(ctx, checkInInfo, time.Now().UTC(), overlay.NodeSelectionConfig{})
			require.NoError(t, err)
		}
		// none of them should be contained
		assertContained(ctx, t, cache, node1, false, node2, false, node3, false)

		// Set node2 (only) to be contained
		err := cache.SetAllContainedNodes(ctx, []storj.NodeID{node2})
		require.NoError(t, err)
		assertContained(ctx, t, cache, node1, false, node2, true, node3, false)

		// Set node1 and node3 (only) to be contained
		err = cache.SetAllContainedNodes(ctx, []storj.NodeID{node1, node3})
		require.NoError(t, err)
		assertContained(ctx, t, cache, node1, true, node2, false, node3, true)

		// Set node1 (only) to be contained
		err = cache.SetAllContainedNodes(ctx, []storj.NodeID{node1})
		require.NoError(t, err)
		assertContained(ctx, t, cache, node1, true, node2, false, node3, false)

		// Set no nodes to be contained
		err = cache.SetAllContainedNodes(ctx, []storj.NodeID{})
		require.NoError(t, err)
		assertContained(ctx, t, cache, node1, false, node2, false, node3, false)
	})
}

func assertContained(ctx context.Context, t testing.TB, cache overlay.DB, args ...interface{}) {
	require.Equal(t, 0, len(args)%2, "must be given an even number of args")
	for n := 0; n < len(args); n += 2 {
		nodeID := args[n].(storj.NodeID)
		expectedContainment := args[n+1].(bool)
		nodeInDB, err := cache.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Equalf(t, expectedContainment, nodeInDB.Contained,
			"Expected nodeID %v (args[%d]) contained = %v, but got %v",
			nodeID, n, expectedContainment, nodeInDB.Contained)
	}
}

func TestGetNodesNetwork(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()
		const (
			distinctNetworks = 10
			netMask          = 28
			nodesPerNetwork  = 1 << (32 - netMask)
		)
		mask := net.CIDRMask(netMask, 32)

		nodes := make([]storj.NodeID, distinctNetworks*nodesPerNetwork)
		ips := make([]net.IP, len(nodes))
		lastNets := make([]string, len(nodes))
		setOfNets := make(map[string]struct{})

		for n := range nodes {
			nodes[n] = testrand.NodeID()
			ips[n] = make(net.IP, 4)
			binary.BigEndian.PutUint32(ips[n], uint32(n))
			lastNets[n] = ips[n].Mask(mask).String()
			setOfNets[lastNets[n]] = struct{}{}

			checkInInfo := overlay.NodeCheckInInfo{
				IsUp:    true,
				Address: &pb.NodeAddress{Address: ips[n].String()},
				LastNet: lastNets[n],
				Version: &pb.NodeVersion{Version: "v0.0.0"},
				NodeID:  nodes[n],
			}
			err := cache.UpdateCheckIn(ctx, checkInInfo, time.Now().UTC(), overlay.NodeSelectionConfig{})
			require.NoError(t, err)
		}

		t.Run("GetNodesNetwork", func(t *testing.T) {
			gotLastNets, err := cache.GetNodesNetwork(ctx, nodes)
			require.NoError(t, err)
			require.Len(t, gotLastNets, len(nodes))
			gotLastNetsSet := make(map[string]struct{})
			for _, lastNet := range gotLastNets {
				gotLastNetsSet[lastNet] = struct{}{}
			}
			require.Len(t, gotLastNetsSet, distinctNetworks)
			for _, lastNet := range gotLastNets {
				require.NotEmpty(t, lastNet)
				delete(setOfNets, lastNet)
			}
			require.Empty(t, setOfNets) // indicates that all last_nets were seen in the result
		})

		t.Run("GetNodesNetworkInOrder", func(t *testing.T) {
			nodesPlusOne := make([]storj.NodeID, len(nodes)+1)
			copy(nodesPlusOne[:len(nodes)], nodes)
			lastNetsPlusOne := make([]string, len(nodes)+1)
			copy(lastNetsPlusOne[:len(nodes)], lastNets)
			// add a node that the overlay cache doesn't know about
			unknownNode := testrand.NodeID()
			nodesPlusOne[len(nodes)] = unknownNode
			lastNetsPlusOne[len(nodes)] = ""

			// shuffle the order of the requested nodes, so we know output is in the right order
			rand.Shuffle(len(nodesPlusOne), func(i, j int) {
				nodesPlusOne[i], nodesPlusOne[j] = nodesPlusOne[j], nodesPlusOne[i]
				lastNetsPlusOne[i], lastNetsPlusOne[j] = lastNetsPlusOne[j], lastNetsPlusOne[i]
			})

			gotLastNets, err := cache.GetNodesNetworkInOrder(ctx, nodesPlusOne)
			require.NoError(t, err)
			require.Len(t, gotLastNets, len(nodes)+1)

			require.Equal(t, lastNetsPlusOne, gotLastNets)
		})
	})
}

func TestOverlayCache_SelectAllStorageNodesDownloadUpload(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()
		const netMask = 28
		mask := net.CIDRMask(netMask, 32)

		infos := make([]overlay.NodeCheckInInfo, 5)

		for n := range infos {
			id := testrand.NodeID()
			ip := net.IP{0, 0, 1, byte(n)}
			lastNet := ip.Mask(mask).String()

			infos[n] = overlay.NodeCheckInInfo{
				IsUp:        true,
				Address:     &pb.NodeAddress{Address: ip.String()},
				LastNet:     lastNet,
				LastIPPort:  "0.0.0.0:0",
				Version:     &pb.NodeVersion{Version: "v0.0.0"},
				NodeID:      id,
				CountryCode: location.Canada,
			}
			err := cache.UpdateCheckIn(ctx, infos[n], time.Now().UTC(), overlay.NodeSelectionConfig{})
			require.NoError(t, err)
		}

		checkNodes := func(selectedNodes []*uploadselection.SelectedNode) {
			selectedNodesMap := map[storj.NodeID]*uploadselection.SelectedNode{}
			for _, node := range selectedNodes {
				selectedNodesMap[node.ID] = node
			}

			for _, info := range infos {
				selectedNode, ok := selectedNodesMap[info.NodeID]
				require.True(t, ok)

				require.Equal(t, info.NodeID, selectedNode.ID)
				require.Equal(t, info.Address, selectedNode.Address)
				require.Equal(t, info.CountryCode, selectedNode.CountryCode)
				require.Equal(t, info.LastIPPort, selectedNode.LastIPPort)
				require.Equal(t, info.LastNet, selectedNode.LastNet)
			}
		}

		selectedNodes, err := cache.SelectAllStorageNodesDownload(ctx, time.Minute, overlay.AsOfSystemTimeConfig{})
		require.NoError(t, err)

		checkNodes(selectedNodes)

		reputableNodes, newNodes, err := cache.SelectAllStorageNodesUpload(ctx, overlay.NodeSelectionConfig{
			OnlineWindow: time.Minute,
		})
		require.NoError(t, err)

		checkNodes(append(reputableNodes, newNodes...))
	})

}

func TestOverlayCache_KnownReliable(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		allNodes := []uploadselection.SelectedNode{
			addNode(ctx, t, cache, "online", "127.0.0.1", true, false, false, false, false),
			addNode(ctx, t, cache, "offline", "127.0.0.2", false, false, false, false, false),
			addNode(ctx, t, cache, "disqalified", "127.0.0.3", false, true, false, false, false),
			addNode(ctx, t, cache, "audit-suspended", "127.0.0.4", false, false, true, false, false),
			addNode(ctx, t, cache, "offline-suspended", "127.0.0.5", false, false, false, true, false),
			addNode(ctx, t, cache, "exited", "127.0.0.6", false, false, false, false, true),
		}

		ids := func(nodes ...uploadselection.SelectedNode) storj.NodeIDList {
			nodeIds := storj.NodeIDList{}
			for _, node := range nodes {
				nodeIds = append(nodeIds, node.ID)
			}
			return nodeIds
		}

		nodes := func(nodes ...uploadselection.SelectedNode) []uploadselection.SelectedNode {
			return append([]uploadselection.SelectedNode{}, nodes...)
		}

		type testCase struct {
			IDs     storj.NodeIDList
			Online  []uploadselection.SelectedNode
			Offline []uploadselection.SelectedNode
		}

		shuffledNodeIDs := ids(allNodes...)
		rand.Shuffle(len(shuffledNodeIDs), shuffledNodeIDs.Swap)

		for _, tc := range []testCase{
			{
				IDs:     ids(allNodes[0], allNodes[1]),
				Online:  nodes(allNodes[0]),
				Offline: nodes(allNodes[1]),
			},
			{
				IDs:    ids(allNodes[0]),
				Online: nodes(allNodes[0]),
			},
			{
				IDs:     ids(allNodes[1]),
				Offline: nodes(allNodes[1]),
			},
			{ // only unreliable
				IDs: ids(allNodes[2], allNodes[3], allNodes[4], allNodes[5]),
			},

			{ // all nodes
				IDs:     ids(allNodes...),
				Online:  nodes(allNodes[0]),
				Offline: nodes(allNodes[1]),
			},
			// all nodes but in shuffled order
			{
				IDs:     shuffledNodeIDs,
				Online:  nodes(allNodes[0]),
				Offline: nodes(allNodes[1]),
			},
			// all nodes + one ID not from DB
			{
				IDs:     append(ids(allNodes...), testrand.NodeID()),
				Online:  nodes(allNodes[0]),
				Offline: nodes(allNodes[1]),
			},
		} {
			online, offline, err := cache.KnownReliable(ctx, tc.IDs, 1*time.Hour, 0)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.Online, online)
			require.ElementsMatch(t, tc.Offline, offline)
		}

		_, _, err := cache.KnownReliable(ctx, storj.NodeIDList{}, 1*time.Hour, 0)
		require.Error(t, err)
	})
}

func addNode(ctx context.Context, t *testing.T, cache overlay.DB, address, lastIPPort string, online, disqalified, auditSuspended, offlineSuspended, exited bool) uploadselection.SelectedNode {
	selectedNode := uploadselection.SelectedNode{
		ID:          testrand.NodeID(),
		Address:     &pb.NodeAddress{Address: address},
		LastNet:     lastIPPort,
		LastIPPort:  lastIPPort,
		CountryCode: location.Poland,
	}

	checkInInfo := overlay.NodeCheckInInfo{
		IsUp:        true,
		NodeID:      selectedNode.ID,
		Address:     &pb.NodeAddress{Address: selectedNode.Address.Address},
		LastIPPort:  selectedNode.LastIPPort,
		LastNet:     selectedNode.LastNet,
		CountryCode: selectedNode.CountryCode,
		Version:     &pb.NodeVersion{Version: "v0.0.0"},
	}

	timestamp := time.Now().UTC()
	if !online {
		timestamp = time.Now().Add(-10 * time.Hour)
	}

	err := cache.UpdateCheckIn(ctx, checkInInfo, timestamp, overlay.NodeSelectionConfig{})
	require.NoError(t, err)

	if disqalified {
		_, err := cache.DisqualifyNode(ctx, selectedNode.ID, time.Now(), overlay.DisqualificationReasonAuditFailure)
		require.NoError(t, err)
	}

	if auditSuspended {
		require.NoError(t, cache.TestSuspendNodeUnknownAudit(ctx, selectedNode.ID, time.Now()))
	}

	if offlineSuspended {
		require.NoError(t, cache.TestSuspendNodeOffline(ctx, selectedNode.ID, time.Now()))
	}

	if exited {
		now := time.Now()
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              selectedNode.ID,
			ExitInitiatedAt:     now,
			ExitLoopCompletedAt: now,
			ExitFinishedAt:      now,
			ExitSuccess:         true,
		})
		require.NoError(t, err)
	}

	return selectedNode
}
