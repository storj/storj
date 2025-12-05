// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"context"
	"fmt"
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
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/version"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/location"
)

func TestGetOfflineNodesForEmail(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		selectionCfg := overlay.NodeSelectionConfig{
			OnlineWindow: 4 * time.Hour,
		}

		offlineID := testrand.NodeID()
		onlineID := testrand.NodeID()
		disqualifiedID := testrand.NodeID()
		exitedID := testrand.NodeID()
		offlineNoEmailID := testrand.NodeID()

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

		nodeID0 := testrand.NodeID()
		nodeID1 := testrand.NodeID()

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
		require.WithinDuration(t, now.Truncate(time.Second), node0.LastOfflineEmail.Truncate(time.Second), time.Nanosecond)

		node1, err := cache.Get(ctx, nodeID1)
		require.NoError(t, err)
		require.WithinDuration(t, now.Truncate(time.Second), node1.LastOfflineEmail.Truncate(time.Second), time.Nanosecond)
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
				_, err = cache.TestVetNode(ctx, id)
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
					require.True(t, selectedNode.Vetted)
				} else {
					require.Len(t, selectedNode.Tags, 0)
					require.False(t, selectedNode.Vetted)
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
	email            string
	wallet           string
	lastIPPort       string
	offlineInterval  time.Duration
	countryCode      location.CountryCode
	disqualified     bool
	auditSuspended   bool
	offlineSuspended bool
	exiting          bool
	exited           bool
	vetted           bool
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
			selectedNodes, err := cache.GetParticipatingNodes(ctx, ids, 1*time.Hour, 0)
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
		_, err := cache.GetParticipatingNodes(ctx, storj.NodeIDList{}, 1*time.Hour, 0)
		require.Error(t, err)

		// test as of system time
		allIDs := make([]storj.NodeID, len(allNodes))
		for i := range allNodes {
			allIDs[i] = allNodes[i].id
		}

		selection, err := cache.GetParticipatingNodes(ctx, allIDs, 1*time.Hour, -1*time.Microsecond)
		require.NoError(t, err)

		require.Equal(t, "0x9b7488BF8b6A4FF21D610e3dd202723f705cD1C0", selection[0].Wallet)
		require.Equal(t, "test@storj.io", selection[0].Email)
		require.True(t, selection[0].Vetted)
	})
}

func TestOverlayCache_GetAllParticipatingNodes(t *testing.T) {
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
			gotNodes, err := cache.GetAllParticipatingNodes(ctx, tc.OnlineWindow, 0)
			require.NoError(t, err)
			require.ElementsMatch(t, expectedNodes, gotNodes, "#%d", i)
		}

		// test as of system time
		selection, err := cache.GetAllParticipatingNodes(ctx, 1*time.Hour, -1*time.Microsecond)
		require.NoError(t, err)

		require.Equal(t, "0x9b7488BF8b6A4FF21D610e3dd202723f705cD1C0", selection[0].Wallet)
		require.Equal(t, "test@storj.io", selection[0].Email)
		require.True(t, selection[0].Vetted)
	})
}

func nodeDispositionToSelectedNode(disp nodeDisposition, onlineWindow time.Duration) nodeselection.SelectedNode {
	if disp.exited || disp.disqualified {
		return nodeselection.SelectedNode{}
	}
	return nodeselection.SelectedNode{
		ID:          disp.id,
		Address:     &pb.NodeAddress{Address: disp.address},
		Email:       disp.email,
		Wallet:      disp.wallet,
		LastNet:     disp.lastIPPort,
		LastIPPort:  disp.lastIPPort,
		CountryCode: disp.countryCode,
		Exiting:     disp.exiting,
		Suspended:   disp.auditSuspended || disp.offlineSuspended,
		Online:      disp.offlineInterval <= onlineWindow,
		Vetted:      disp.vetted,
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
		email:            "test@storj.io",
		wallet:           "0x9b7488BF8b6A4FF21D610e3dd202723f705cD1C0",
		vetted:           true,
	}

	checkInInfo := overlay.NodeCheckInInfo{
		IsUp:        true,
		NodeID:      disp.id,
		Address:     &pb.NodeAddress{Address: disp.address},
		LastIPPort:  disp.lastIPPort,
		LastNet:     disp.lastIPPort,
		CountryCode: disp.countryCode,
		Version:     &pb.NodeVersion{Version: "v0.0.0"},
		Operator: &pb.NodeOperator{
			Email:  disp.email,
			Wallet: disp.wallet,
		},
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

	if disp.vetted {
		_, err = cache.TestVetNode(ctx, disp.id)
		require.NoError(t, err)
	}

	return disp
}

func TestOverlayCache_KnownReliableTagHandling(t *testing.T) {
	signer := testidentity.MustPregeneratedIdentity(0, storj.LatestIDVersion())

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {

		cache := db.OverlayCache()

		// GIVEN

		var ids []storj.NodeID
		for i := 0; i < 10; i++ {
			address := fmt.Sprintf("127.0.0.%d", i)
			checkInInfo := overlay.NodeCheckInInfo{
				IsUp:        true,
				NodeID:      testidentity.MustPregeneratedIdentity(i+1, storj.LatestIDVersion()).ID,
				Address:     &pb.NodeAddress{Address: address},
				LastIPPort:  address + ":1234",
				LastNet:     "127.0.0.0",
				CountryCode: location.Romania,
				Version:     &pb.NodeVersion{Version: "v0.0.0"},
			}

			ids = append(ids, checkInInfo.NodeID)

			checkin := time.Now().UTC()
			if i%2 == 0 {
				checkin = checkin.Add(-50 * time.Hour)
			}
			err := cache.UpdateCheckIn(ctx, checkInInfo, checkin, overlay.NodeSelectionConfig{})
			require.NoError(t, err)

			tags := nodeselection.NodeTags{}

			if i%2 == 0 {
				tags = append(tags, nodeselection.NodeTag{
					SignedAt: time.Now(),
					Signer:   signer.ID,
					NodeID:   checkInInfo.NodeID,
					Name:     "index",
					Value:    []byte{byte(i)},
				})
			}
			if i%4 == 0 {
				tags = append(tags, nodeselection.NodeTag{
					SignedAt: time.Now(),
					Signer:   signer.ID,
					NodeID:   checkInInfo.NodeID,
					Name:     "selected",
					Value:    []byte("true"),
				})
			}

			if len(tags) > 0 {
				require.NoError(t, err)
				err = cache.UpdateNodeTags(ctx, tags)
				require.NoError(t, err)
			}

		}

		// WHEN
		nodes, err := cache.GetParticipatingNodes(ctx, ids, 1*time.Hour, 0)
		require.NoError(t, err)

		// THEN
		require.Len(t, nodes, 10)

		checkTag := func(tags nodeselection.NodeTags, name string, value []byte) {
			tag1, err := tags.FindBySignerAndName(signer.ID, name)
			require.NoError(t, err)
			require.Equal(t, name, tag1.Name)
			require.Equal(t, value, tag1.Value)
			require.Equal(t, signer.ID, tag1.Signer)
			require.True(t, time.Since(tag1.SignedAt) < 1*time.Hour)
		}

		for _, node := range nodes {
			ipParts := strings.Split(node.Address.Address, ".")
			ix, err := strconv.Atoi(ipParts[3])
			require.NoError(t, err)

			if ix%4 == 0 {
				require.Len(t, node.Tags, 2)
				checkTag(node.Tags, "selected", []byte("true"))
				checkTag(node.Tags, "index", []byte{byte(ix)})
			} else if ix%2 == 0 {
				checkTag(node.Tags, "index", []byte{byte(ix)})
				require.Len(t, node.Tags, 1)
			} else {
				require.Len(t, node.Tags, 0)
			}
		}
	})
}

func TestOverlayCache_GetLastIPPortByNodeTagName(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		var ids storj.NodeIDList
		for i := 0; i < 6; i++ {
			ids = append(ids, testrand.NodeID())
		}
		lastIPPorts := []string{"127.0.0.1:0", "127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "127.0.0.1:4", ""}
		tagNames := []string{"test-tag-name-1", "test-tag-name-2"}

		for i, id := range ids {
			require.NoError(t, cache.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
				NodeID:     id,
				Address:    &pb.NodeAddress{Address: "127.0.0.1"},
				LastIPPort: lastIPPorts[i],
				LastNet:    "127.0.0",
				Version:    &pb.NodeVersion{Version: "v1.0.0"},
				IsUp:       true,
			}, time.Now(), overlay.NodeSelectionConfig{}))
		}

		require.NoError(t, cache.UpdateNodeTags(ctx, nodeselection.NodeTags{
			{
				NodeID:   ids[0],
				SignedAt: time.Now(),
				Signer:   ids[0],
				Name:     tagNames[0],
				Value:    []byte("testvalue"),
			},
			{
				NodeID:   ids[1],
				SignedAt: time.Now(),
				Signer:   ids[1],
				Name:     tagNames[0],
				Value:    []byte("testvalue"),
			},
			{
				NodeID:   ids[5],
				SignedAt: time.Now(),
				Signer:   ids[5],
				Name:     tagNames[0],
				Value:    []byte("testvalue"),
			},
			{
				NodeID:   ids[2],
				SignedAt: time.Now(),
				Signer:   ids[2],
				Name:     "some-other-tag",
				Value:    []byte("testvalue"),
			},
			{
				NodeID:   ids[3],
				SignedAt: time.Now(),
				Signer:   ids[3],
				Name:     tagNames[1],
				Value:    []byte("testvalue"),
			},
		}))

		queriedLastIPPorts, err := cache.GetLastIPPortByNodeTagNames(ctx, ids, tagNames)
		require.NoError(t, err)
		require.Len(t, queriedLastIPPorts, 3)

		lastIPPort, ok := queriedLastIPPorts[ids[0]]
		require.True(t, ok)
		require.NotNil(t, lastIPPort)
		require.Equal(t, lastIPPorts[0], *lastIPPort)

		lastIPPort, ok = queriedLastIPPorts[ids[1]]
		require.True(t, ok)
		require.NotNil(t, lastIPPort)
		require.Equal(t, lastIPPorts[1], *lastIPPort)

		lastIPPort, ok = queriedLastIPPorts[ids[3]]
		require.True(t, ok)
		require.NotNil(t, lastIPPort)
		require.Equal(t, lastIPPorts[3], *lastIPPort)

		_, ok = queriedLastIPPorts[ids[2]]
		require.False(t, ok)

		_, ok = queriedLastIPPorts[ids[4]]
		require.False(t, ok)

		_, ok = queriedLastIPPorts[ids[5]]
		require.False(t, ok)
	})
}

func TestOverlayCache_ActiveNodesPieceCounts(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		overlay := db.OverlayCache()

		onlineNode := addNode(ctx, t, overlay, "online           ", "127.0.0.1", time.Second, false, false, false, false, false)
		offlineNode := addNode(ctx, t, overlay, "offline          ", "127.0.0.2", 2*time.Hour, false, false, false, false, false)

		addNode(ctx, t, overlay, "disqualified     ", "127.0.0.3", 2*time.Hour, true, false, false, false, false)
		addNode(ctx, t, overlay, "exiting          ", "127.0.0.5", 2*time.Hour, false, false, false, true, false)
		addNode(ctx, t, overlay, "exited           ", "127.0.0.6", 2*time.Hour, false, false, false, false, true)

		nodes, err := overlay.ActiveNodesPieceCounts(ctx)
		require.NoError(t, err)

		require.Len(t, nodes, 2)

		_, found := nodes[onlineNode.id]
		require.True(t, found)

		_, found = nodes[offlineNode.id]
		require.True(t, found)
	})
}
