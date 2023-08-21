// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"context"
	"encoding/binary"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/private/version"
	"storj.io/storj/private/teststorj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/nodeselection"
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
		tagSigner := testidentity.MustPregeneratedIdentity(0, storj.LatestIDVersion())

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

			if n%2 == 0 {
				err = cache.UpdateNodeTags(ctx, nodeselection.NodeTags{
					nodeselection.NodeTag{
						NodeID:   id,
						SignedAt: time.Now(),
						Signer:   tagSigner.ID,
						Name:     "even",
						Value:    []byte{1},
					},
				})
				require.NoError(t, err)
			}
		}

		checkNodes := func(selectedNodes []*nodeselection.SelectedNode) {
			selectedNodesMap := map[storj.NodeID]*nodeselection.SelectedNode{}
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
				segments := strings.Split(selectedNode.Address.Address, ".")
				origIndex, err := strconv.Atoi(segments[len(segments)-1])
				require.NoError(t, err)
				if origIndex%2 == 0 {
					require.Len(t, selectedNode.Tags, 1)
					require.Equal(t, "even", selectedNode.Tags[0].Name)
					require.Equal(t, []byte{1}, selectedNode.Tags[0].Value)
				} else {
					require.Len(t, selectedNode.Tags, 0)
				}
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

type nodeDisposition struct {
	id               storj.NodeID
	address          string
	lastIPPort       string
	offlineInterval  time.Duration
	countryCode      location.CountryCode
	disqualified     bool
	auditSuspended   bool
	offlineSuspended bool
	exiting          bool
	exited           bool
}

func TestOverlayCache_GetNodes(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		allNodes := []nodeDisposition{
			addNode(ctx, t, cache, "online           ", "127.0.0.1", time.Second, false, false, false, false, false),
			addNode(ctx, t, cache, "offline          ", "127.0.0.2", 2*time.Hour, false, false, false, false, false),
			addNode(ctx, t, cache, "disqualified     ", "127.0.0.3", 2*time.Hour, true, false, false, false, false),
			addNode(ctx, t, cache, "audit-suspended  ", "127.0.0.4", time.Second, false, true, false, false, false),
			addNode(ctx, t, cache, "offline-suspended", "127.0.0.5", 2*time.Hour, false, false, true, false, false),
			addNode(ctx, t, cache, "exiting          ", "127.0.0.5", 2*time.Hour, false, false, false, true, false),
			addNode(ctx, t, cache, "exited           ", "127.0.0.6", 2*time.Hour, false, false, false, false, true),
		}

		nodes := func(nodeNums ...int) []nodeDisposition {
			nodeDisps := make([]nodeDisposition, len(nodeNums))
			for i, nodeNum := range nodeNums {
				nodeDisps[i] = allNodes[nodeNum]
			}
			return nodeDisps
		}

		sNodes := func(nodes ...int) []nodeselection.SelectedNode {
			selectedNodes := make([]nodeselection.SelectedNode, len(nodes))
			for i, nodeNum := range nodes {
				selectedNodes[i] = nodeDispositionToSelectedNode(allNodes[nodeNum], time.Hour)
			}
			return selectedNodes
		}

		type testCase struct {
			QueryNodes []nodeDisposition
			Online     []nodeselection.SelectedNode
			Offline    []nodeselection.SelectedNode
		}

		for testNum, tc := range []testCase{
			{
				QueryNodes: nodes(0, 1),
				Online:     sNodes(0),
				Offline:    sNodes(1),
			},
			{
				QueryNodes: nodes(0),
				Online:     sNodes(0),
			},
			{
				QueryNodes: nodes(1),
				Offline:    sNodes(1),
			},
			{ // only unreliable
				QueryNodes: nodes(2, 3, 4, 5),
				Online:     sNodes(3),
				Offline:    sNodes(4, 5),
			},

			{ // all nodes
				QueryNodes: allNodes,
				Online:     sNodes(0, 3),
				Offline:    sNodes(1, 4, 5),
			},
			// all nodes + one ID not from DB
			{
				QueryNodes: append(allNodes, nodeDisposition{
					id:           testrand.NodeID(),
					disqualified: true, // just so we expect a zero ID for this entry
				}),
				Online:  sNodes(0, 3),
				Offline: sNodes(1, 4, 5),
			},
		} {
			ids := make([]storj.NodeID, len(tc.QueryNodes))
			for i := range tc.QueryNodes {
				ids[i] = tc.QueryNodes[i].id
			}
			selectedNodes, err := cache.GetNodes(ctx, ids, 1*time.Hour, 0)
			require.NoError(t, err)
			require.Equal(t, len(tc.QueryNodes), len(selectedNodes))
			var gotOnline []nodeselection.SelectedNode
			var gotOffline []nodeselection.SelectedNode
			for i, n := range selectedNodes {
				if tc.QueryNodes[i].disqualified || tc.QueryNodes[i].exited {
					assert.Zero(t, n, testNum, i)
				} else {
					assert.Equal(t, tc.QueryNodes[i].id, selectedNodes[i].ID, "%d:%d", testNum, i)
					if n.Online {
						gotOnline = append(gotOnline, n)
					} else {
						gotOffline = append(gotOffline, n)
					}
				}
			}
			assert.Equal(t, tc.Online, gotOnline)
			assert.Equal(t, tc.Offline, gotOffline)
		}

		// test empty id list
		_, err := cache.GetNodes(ctx, storj.NodeIDList{}, 1*time.Hour, 0)
		require.Error(t, err)

		// test as of system time
		allIDs := make([]storj.NodeID, len(allNodes))
		for i := range allNodes {
			allIDs[i] = allNodes[i].id
		}
		_, err = cache.GetNodes(ctx, allIDs, 1*time.Hour, -1*time.Microsecond)
		require.NoError(t, err)
	})
}

func TestOverlayCache_GetParticipatingNodes(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		allNodes := []nodeDisposition{
			addNode(ctx, t, cache, "online           ", "127.0.0.1", time.Second, false, false, false, false, false),
			addNode(ctx, t, cache, "offline          ", "127.0.0.2", 2*time.Hour, false, false, false, false, false),
			addNode(ctx, t, cache, "disqualified     ", "127.0.0.3", 2*time.Hour, true, false, false, false, false),
			addNode(ctx, t, cache, "audit-suspended  ", "127.0.0.4", time.Second, false, true, false, false, false),
			addNode(ctx, t, cache, "offline-suspended", "127.0.0.5", 2*time.Hour, false, false, true, false, false),
			addNode(ctx, t, cache, "exiting          ", "127.0.0.5", 2*time.Hour, false, false, false, true, false),
			addNode(ctx, t, cache, "exited           ", "127.0.0.6", 2*time.Hour, false, false, false, false, true),
		}

		type testCase struct {
			OnlineWindow time.Duration
			Online       []int
			Offline      []int
		}

		for i, tc := range []testCase{
			{
				OnlineWindow: 1 * time.Hour,
				Online:       []int{0, 3},
				Offline:      []int{1, 4, 5},
			},
			{
				OnlineWindow: 20 * time.Hour,
				Online:       []int{0, 1, 3, 4, 5},
			},
			{
				OnlineWindow: 1 * time.Microsecond,
				Offline:      []int{0, 1, 3, 4, 5},
			},
		} {
			expectedNodes := make([]nodeselection.SelectedNode, 0, len(tc.Offline)+len(tc.Online))
			for _, num := range tc.Online {
				selectedNode := nodeDispositionToSelectedNode(allNodes[num], 0)
				selectedNode.Online = true
				expectedNodes = append(expectedNodes, selectedNode)
			}
			for _, num := range tc.Offline {
				selectedNode := nodeDispositionToSelectedNode(allNodes[num], 0)
				selectedNode.Online = false
				expectedNodes = append(expectedNodes, selectedNode)
			}
			gotNodes, err := cache.GetParticipatingNodes(ctx, tc.OnlineWindow, 0)
			require.NoError(t, err)
			require.ElementsMatch(t, expectedNodes, gotNodes, "#%d", i)
		}

		// test as of system time
		_, err := cache.GetParticipatingNodes(ctx, 1*time.Hour, -1*time.Microsecond)
		require.NoError(t, err)
	})
}

func nodeDispositionToSelectedNode(disp nodeDisposition, onlineWindow time.Duration) nodeselection.SelectedNode {
	if disp.exited || disp.disqualified {
		return nodeselection.SelectedNode{}
	}
	return nodeselection.SelectedNode{
		ID:          disp.id,
		Address:     &pb.NodeAddress{Address: disp.address},
		LastNet:     disp.lastIPPort,
		LastIPPort:  disp.lastIPPort,
		CountryCode: disp.countryCode,
		Exiting:     disp.exiting,
		Suspended:   disp.auditSuspended || disp.offlineSuspended,
		Online:      disp.offlineInterval <= onlineWindow,
	}
}

func addNode(ctx context.Context, t *testing.T, cache overlay.DB, address, lastIPPort string, offlineInterval time.Duration, disqualified, auditSuspended, offlineSuspended, exiting, exited bool) nodeDisposition {
	disp := nodeDisposition{
		id:               testrand.NodeID(),
		address:          address,
		lastIPPort:       lastIPPort,
		offlineInterval:  offlineInterval,
		countryCode:      location.Poland,
		disqualified:     disqualified,
		auditSuspended:   auditSuspended,
		offlineSuspended: offlineSuspended,
		exiting:          exiting,
		exited:           exited,
	}

	checkInInfo := overlay.NodeCheckInInfo{
		IsUp:        true,
		NodeID:      disp.id,
		Address:     &pb.NodeAddress{Address: disp.address},
		LastIPPort:  disp.lastIPPort,
		LastNet:     disp.lastIPPort,
		CountryCode: disp.countryCode,
		Version:     &pb.NodeVersion{Version: "v0.0.0"},
	}

	timestamp := time.Now().UTC().Add(-disp.offlineInterval)

	err := cache.UpdateCheckIn(ctx, checkInInfo, timestamp, overlay.NodeSelectionConfig{})
	require.NoError(t, err)

	if disqualified {
		_, err := cache.DisqualifyNode(ctx, disp.id, time.Now(), overlay.DisqualificationReasonAuditFailure)
		require.NoError(t, err)
	}

	if auditSuspended {
		require.NoError(t, cache.TestSuspendNodeUnknownAudit(ctx, disp.id, time.Now()))
	}

	if offlineSuspended {
		require.NoError(t, cache.TestSuspendNodeOffline(ctx, disp.id, time.Now()))
	}

	if exiting {
		now := time.Now()
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:          disp.id,
			ExitInitiatedAt: now,
		})
		require.NoError(t, err)
	}

	if exited {
		now := time.Now()
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              disp.id,
			ExitInitiatedAt:     now,
			ExitLoopCompletedAt: now,
			ExitFinishedAt:      now,
			ExitSuccess:         true,
		})
		require.NoError(t, err)
	}

	return disp
}
