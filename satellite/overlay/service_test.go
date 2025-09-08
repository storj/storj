// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestCache_Database(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		testCache(ctx, t, db.OverlayCache(), db.NodeEvents())
	})
}

// returns a NodeSelectionConfig with sensible test values.
func testNodeSelectionConfig(newNodeFraction float64) overlay.NodeSelectionConfig {
	return overlay.NodeSelectionConfig{
		NewNodeFraction: newNodeFraction,
		OnlineWindow:    time.Hour,
	}
}

// returns an AuditHistoryConfig with sensible test values.
func testAuditHistoryConfig() reputation.AuditHistoryConfig {
	return reputation.AuditHistoryConfig{
		WindowSize:       time.Hour,
		TrackingPeriod:   time.Hour,
		GracePeriod:      time.Hour,
		OfflineThreshold: 0,
	}
}

func testCache(ctx *testcontext.Context, t *testing.T, store overlay.DB, nodeEvents nodeevents.DB) {
	valid1ID := testrand.NodeID()
	valid2ID := testrand.NodeID()
	valid3ID := testrand.NodeID()
	missingID := testrand.NodeID()
	address := &pb.NodeAddress{Address: "127.0.0.1:0"}
	lastNet := "127.0.0"

	nodeSelectionConfig := testNodeSelectionConfig(0)
	serviceConfig := overlay.Config{
		Node: nodeSelectionConfig,
		NodeSelectionCache: overlay.UploadSelectionCacheConfig{
			Staleness: lowStaleness,
		},
		UpdateStatsBatchSize: 100,
	}

	serviceCtx, serviceCancel := context.WithCancel(ctx)
	defer serviceCancel()
	service, err := overlay.NewService(zaptest.NewLogger(t), store, nodeEvents, nodeselection.TestPlacementDefinitions(), "", "", serviceConfig)
	require.NoError(t, err)
	ctx.Go(func() error { return service.Run(serviceCtx) })
	defer ctx.Check(service.Close)

	d := overlay.NodeCheckInInfo{
		Address:    address,
		LastIPPort: address.Address,
		LastNet:    lastNet,
		Version:    &pb.NodeVersion{Version: "v1.0.0"},
		IsUp:       true,
	}
	{ // Put
		d.NodeID = valid1ID
		err := store.UpdateCheckIn(ctx, d, time.Now().UTC(), nodeSelectionConfig)
		require.NoError(t, err)

		d.NodeID = valid2ID
		err = store.UpdateCheckIn(ctx, d, time.Now().UTC(), nodeSelectionConfig)
		require.NoError(t, err)

		d.NodeID = valid3ID
		err = store.UpdateCheckIn(ctx, d, time.Now().UTC(), nodeSelectionConfig)
		require.NoError(t, err)
		// disqualify one node
		err = service.DisqualifyNode(ctx, valid3ID, overlay.DisqualificationReasonUnknown)
		require.NoError(t, err)
	}

	{ // Invalid shouldn't cause a panic.
		validInfo := func() overlay.NodeCheckInInfo {
			return overlay.NodeCheckInInfo{
				Address:    address,
				LastIPPort: address.Address,
				LastNet:    lastNet,
				Version: &pb.NodeVersion{
					Version:    "v1.0.0",
					CommitHash: "alpha",
				},
				IsUp: true,
				Operator: &pb.NodeOperator{
					Email:          "\x00",
					Wallet:         "0x1234",
					WalletFeatures: []string{"zerog"},
				},
			}
		}

		// Currently Postgres returns an error and CockroachDB doesn't return
		// an error for a non-utf text field.

		d := validInfo()
		d.Operator.Email = "\x00"
		_ = store.UpdateCheckIn(ctx, d, time.Now().UTC(), nodeSelectionConfig)

		d = validInfo()
		d.Operator.Wallet = "\x00"
		_ = store.UpdateCheckIn(ctx, d, time.Now().UTC(), nodeSelectionConfig)

		d = validInfo()
		d.Operator.WalletFeatures[0] = "\x00"
		_ = store.UpdateCheckIn(ctx, d, time.Now().UTC(), nodeSelectionConfig)

		d = validInfo()
		d.Version.CommitHash = "\x00"
		_ = store.UpdateCheckIn(ctx, d, time.Now().UTC(), nodeSelectionConfig)
	}

	{ // Get
		_, err := service.Get(ctx, storj.NodeID{})
		require.Error(t, err)
		require.Equal(t, overlay.ErrEmptyNode, err)

		valid1, err := service.Get(ctx, valid1ID)
		require.NoError(t, err)
		require.Equal(t, valid1.Id, valid1ID)

		valid2, err := service.Get(ctx, valid2ID)
		require.NoError(t, err)
		require.Equal(t, valid2.Id, valid2ID)

		invalid2, err := service.Get(ctx, missingID)
		require.Error(t, err)
		require.True(t, overlay.ErrNodeNotFound.Has(err))
		require.Nil(t, invalid2)

		// TODO: add erroring database test
	}
}

func TestRandomizedSelectionCache(t *testing.T) {
	totalNodes := 1000
	selectIterations := 100
	numNodesToSelect := 100
	minSelectCount := 3

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.NodeSelectionCache.Staleness = lowStaleness
				config.Overlay.Node.NewNodeFraction = 0.5 // select 50% new nodes
				config.Reputation.AuditCount = 1
				config.Reputation.AuditLambda = 1
				config.Reputation.AuditWeight = 1
				config.Reputation.AuditDQ = 0.5
				config.Reputation.AuditHistory = testAuditHistoryConfig()
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		overlaydb := satellite.Overlay.DB
		uploadSelectionCache := satellite.Overlay.Service.UploadSelectionCache
		allIDs := make(storj.NodeIDList, totalNodes)
		nodeCounts := make(map[storj.NodeID]int)

		var nodes []*overlay.NodeDossier
		now := time.Now()

		// put nodes in cache
		for i := 0; i < totalNodes; i++ {
			newID := testrand.NodeID()
			address := fmt.Sprintf("127.0.%d.0:8080", i)
			lastNet := address

			var vettedAt *time.Time
			if i%2 == 0 {
				vettedAt = &now
			}

			nodes = append(nodes, &overlay.NodeDossier{
				Node: pb.Node{
					Id: newID,
					Address: &pb.NodeAddress{
						Address: address,
					},
				},
				LastNet:    lastNet,
				LastIPPort: address,

				Operator: pb.NodeOperator{},
				Capacity: pb.NodeCapacity{
					FreeDisk: 200 * memory.MiB.Int64(),
				},
				Version: pb.NodeVersion{
					Version:    "v1.1.0",
					CommitHash: "",
					Timestamp:  time.Time{},
					Release:    true,
				},

				Reputation: overlay.NodeStats{
					LastContactSuccess: now,
					Status: overlay.ReputationStatus{
						VettedAt: vettedAt,
					},
				},
			})

			allIDs[i] = newID
			nodeCounts[newID] = 0
		}

		require.NoError(t, overlaydb.TestAddNodes(ctx, nodes))

		err := uploadSelectionCache.Refresh(ctx)
		require.NoError(t, err)

		// select numNodesToSelect nodes selectIterations times
		for i := 0; i < selectIterations; i++ {
			var nodes []*nodeselection.SelectedNode
			var err error
			req := overlay.FindStorageNodesRequest{
				RequestedCount: numNodesToSelect,
			}

			nodes, err = uploadSelectionCache.GetNodes(ctx, req)
			require.NoError(t, err)
			require.Len(t, nodes, numNodesToSelect)

			for _, node := range nodes {
				nodeCounts[node.ID]++
			}
		}

		belowThreshold := 0

		table := []int{}

		// expect that each node has been selected at least minSelectCount times
		for _, id := range allIDs {
			count := nodeCounts[id]
			if count < minSelectCount {
				belowThreshold++
			}
			if count >= len(table) {
				table = append(table, make([]int, count-len(table)+1)...)
			}
			table[count]++
		}

		if belowThreshold > totalNodes*1/100 {
			t.Errorf("%d out of %d were below threshold %d", belowThreshold, totalNodes, minSelectCount)
			for count, amount := range table {
				t.Logf("%3d = %4d", count, amount)
			}
		}
	})
}

func TestNodeInfo(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.StorageNodes[0].Storage2.Monitor.Loop.Pause()

		node, err := planet.Satellites[0].Overlay.Service.Get(ctx, planet.StorageNodes[0].ID())
		require.NoError(t, err)

		dossier := planet.StorageNodes[0].Contact.Service.Local()

		assert.NotEmpty(t, node.Operator.Email)
		assert.NotEmpty(t, node.Operator.Wallet)
		assert.Equal(t, dossier.Operator, node.Operator)
		assert.NotEmpty(t, node.Capacity.FreeDisk)
		assert.Equal(t, dossier.Capacity, node.Capacity)
		assert.NotEmpty(t, node.Version.Version)
		assert.Equal(t, dossier.Version.Version, node.Version.Version)
	})
}

func TestGetNodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.AuditHistory = reputation.AuditHistoryConfig{
					WindowSize:               time.Hour,
					TrackingPeriod:           2 * time.Hour,
					GracePeriod:              time.Hour,
					OfflineThreshold:         0.6,
					OfflineDQEnabled:         false,
					OfflineSuspensionEnabled: true,
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		service := satellite.Overlay.Service
		oc := satellite.DB.OverlayCache()

		// Disqualify storage node #0
		_, err := oc.DisqualifyNode(ctx, planet.StorageNodes[0].ID(), time.Now().UTC(), overlay.DisqualificationReasonUnknown)
		require.NoError(t, err)

		// Stop storage node #1
		offlineNode := planet.StorageNodes[1]
		err = planet.StopPeer(offlineNode)
		require.NoError(t, err)
		// set last contact success to 1 hour ago to make node appear offline
		checkInInfo := getNodeInfo(offlineNode.ID())
		err = service.UpdateCheckIn(ctx, checkInInfo, time.Now().Add(-time.Hour))
		require.NoError(t, err)
		// Check that storage node #1 is offline
		node, err := service.Get(ctx, offlineNode.ID())
		require.NoError(t, err)
		require.False(t, service.IsOnline(node))

		// unknown audit suspend storage node #2
		err = oc.TestSuspendNodeUnknownAudit(ctx, planet.StorageNodes[2].ID(), time.Now())
		require.NoError(t, err)

		// offline suspend storage node #3
		err = oc.TestSuspendNodeOffline(ctx, planet.StorageNodes[3].ID(), time.Now())
		require.NoError(t, err)

		// Check that the results of GetParticipatingNodes match expectations.
		selectedNodes, err := service.GetParticipatingNodes(ctx, []storj.NodeID{
			planet.StorageNodes[0].ID(),
			planet.StorageNodes[1].ID(),
			planet.StorageNodes[2].ID(),
			planet.StorageNodes[3].ID(),
			planet.StorageNodes[4].ID(),
			planet.StorageNodes[5].ID(),
		})
		require.NoError(t, err)
		require.Len(t, selectedNodes, 6)
		require.False(t, selectedNodes[0].Online)
		require.Zero(t, selectedNodes[0]) // node was disqualified
		require.False(t, selectedNodes[1].Online)
		require.False(t, selectedNodes[1].Suspended)
		require.True(t, selectedNodes[2].Online)
		require.True(t, selectedNodes[2].Suspended)
		require.True(t, selectedNodes[3].Online)
		require.True(t, selectedNodes[3].Suspended)
		require.True(t, selectedNodes[4].Online)
		require.False(t, selectedNodes[4].Suspended)
		require.True(t, selectedNodes[5].Online)
		require.False(t, selectedNodes[5].Suspended)

		// Assert the returned nodes are the expected ones
		for i, node := range selectedNodes {
			if i == 0 {
				continue
			}
			assert.Equal(t, planet.StorageNodes[i].ID(), node.ID)
		}
	})
}

func TestUpdateCheckIn(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) { // setup
		nodeID := storj.NodeID{1, 2, 3}
		expectedEmail := "test@email.test"
		expectedAddress := "1.2.4.4:8080"
		info := overlay.NodeCheckInInfo{
			NodeID: nodeID,
			Address: &pb.NodeAddress{
				Address: expectedAddress,
			},
			IsUp: true,
			Capacity: &pb.NodeCapacity{
				FreeDisk: int64(5678),
			},
			Operator: &pb.NodeOperator{
				Email:          expectedEmail,
				Wallet:         "0x123",
				WalletFeatures: []string{"example"},
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			LastIPPort: expectedAddress,
			LastNet:    "1.2.4",
		}
		expectedNode := &overlay.NodeDossier{
			Node: pb.Node{
				Id: nodeID,
				Address: &pb.NodeAddress{
					Address: info.Address.GetAddress(),
				},
			},
			Operator: pb.NodeOperator{
				Email:          info.Operator.GetEmail(),
				Wallet:         info.Operator.GetWallet(),
				WalletFeatures: info.Operator.GetWalletFeatures(),
			},
			Capacity: pb.NodeCapacity{
				FreeDisk: info.Capacity.GetFreeDisk(),
			},
			Version: pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			Reputation: overlay.NodeStats{
				Status: overlay.ReputationStatus{Email: expectedEmail},
			},
			Contained:    false,
			Disqualified: nil,
			PieceCount:   0,
			ExitStatus:   overlay.ExitStatus{NodeID: nodeID},
			LastIPPort:   expectedAddress,
			LastNet:      "1.2.4",
		}

		// confirm the node doesn't exist in nodes table yet
		_, err := db.OverlayCache().Get(ctx, nodeID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "node not found")

		// check-in for that node id, which should add the node
		// to the nodes tables in the database
		startOfTest := time.Now()
		err = db.OverlayCache().UpdateCheckIn(ctx, info, startOfTest.Add(time.Second), overlay.NodeSelectionConfig{})
		require.NoError(t, err)

		// confirm that the node is now in the nodes table with the
		// correct fields set
		actualNode, err := db.OverlayCache().Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, actualNode.Reputation.LastContactSuccess.After(startOfTest))
		require.True(t, actualNode.Reputation.LastContactFailure.UTC().Equal(time.Time{}.UTC()))
		actualNode.Address = expectedNode.Address

		// we need to overwrite the times so that the deep equal considers them the same
		expectedNode.Reputation.LastContactSuccess = actualNode.Reputation.LastContactSuccess
		expectedNode.Reputation.LastContactFailure = actualNode.Reputation.LastContactFailure
		expectedNode.Version.Timestamp = actualNode.Version.Timestamp
		expectedNode.CreatedAt = actualNode.CreatedAt
		require.Equal(t, expectedNode, actualNode)

		// confirm that we can update the address field
		startOfUpdateTest := time.Now()
		expectedAddress = "9.8.7.6"
		updatedInfo := overlay.NodeCheckInInfo{
			NodeID: nodeID,
			Address: &pb.NodeAddress{
				Address: expectedAddress,
			},
			IsUp: true,
			Version: &pb.NodeVersion{
				Version:    "v0.1.0",
				CommitHash: "abc123",
				Timestamp:  time.Now(),
				Release:    true,
			},
			LastIPPort: expectedAddress,
			LastNet:    "9.8.7",
		}
		// confirm that the updated node is in the nodes table with the
		// correct updated fields set
		err = db.OverlayCache().UpdateCheckIn(ctx, updatedInfo, startOfUpdateTest.Add(time.Second), overlay.NodeSelectionConfig{})
		require.NoError(t, err)
		updatedNode, err := db.OverlayCache().Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, updatedNode.Reputation.LastContactSuccess.After(startOfUpdateTest))
		require.True(t, updatedNode.Reputation.LastContactFailure.Equal(time.Time{}))
		require.Equal(t, updatedNode.Address.GetAddress(), expectedAddress)
		require.Equal(t, updatedInfo.Version.GetVersion(), updatedNode.Version.GetVersion())
		require.Equal(t, updatedInfo.Version.GetCommitHash(), updatedNode.Version.GetCommitHash())
		require.Equal(t, updatedInfo.Version.GetRelease(), updatedNode.Version.GetRelease())
		require.True(t, updatedNode.Version.GetTimestamp().After(info.Version.GetTimestamp()))

		// confirm we can udpate IsUp field
		startOfUpdateTest2 := time.Now()
		updatedInfo2 := overlay.NodeCheckInInfo{
			NodeID: nodeID,
			Address: &pb.NodeAddress{
				Address: "9.8.7.6",
			},
			IsUp: false,
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
		}

		err = db.OverlayCache().UpdateCheckIn(ctx, updatedInfo2, startOfUpdateTest2.Add(time.Second), overlay.NodeSelectionConfig{})
		require.NoError(t, err)
		updated2Node, err := db.OverlayCache().Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, updated2Node.Reputation.LastContactSuccess.Equal(updatedNode.Reputation.LastContactSuccess))
		require.True(t, updated2Node.Reputation.LastContactFailure.After(startOfUpdateTest2))

		// check that UpdateCheckIn updates last_offline_email
		require.NoError(t, db.OverlayCache().UpdateLastOfflineEmail(ctx, []storj.NodeID{updated2Node.Id}, time.Now()))
		nodeInfo, err := db.OverlayCache().Get(ctx, updated2Node.Id)
		require.NoError(t, err)
		require.NotNil(t, nodeInfo.LastOfflineEmail)
		lastEmail := nodeInfo.LastOfflineEmail

		// first that it is not updated if node is offline
		require.NoError(t, db.OverlayCache().UpdateCheckIn(ctx, updatedInfo2, time.Now(), overlay.NodeSelectionConfig{}))
		nodeInfo, err = db.OverlayCache().Get(ctx, updated2Node.Id)
		require.NoError(t, err)
		require.Equal(t, lastEmail, nodeInfo.LastOfflineEmail)

		// then that it is nullified if node is online
		updatedInfo2.IsUp = true
		require.NoError(t, db.OverlayCache().UpdateCheckIn(ctx, updatedInfo2, time.Now(), overlay.NodeSelectionConfig{}))
		nodeInfo, err = db.OverlayCache().Get(ctx, updated2Node.Id)
		require.NoError(t, err)
		require.Nil(t, nodeInfo.LastOfflineEmail)
	})
}

func TestUpdateReputation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.Satellites[0].Overlay.Service
		overlaydb := planet.Satellites[0].Overlay.DB
		node := planet.StorageNodes[0]

		info, err := service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.Nil(t, info.Disqualified)
		require.Nil(t, info.UnknownAuditSuspended)
		require.Nil(t, info.OfflineSuspended)
		require.Nil(t, info.Reputation.Status.VettedAt)

		t0 := time.Now().Truncate(time.Hour)
		t1 := t0.Add(time.Hour)
		t2 := t0.Add(2 * time.Hour)
		t3 := t0.Add(3 * time.Hour)

		reputationUpdate := overlay.ReputationUpdate{
			Disqualified:          nil,
			UnknownAuditSuspended: &t1,
			OfflineSuspended:      &t2,
			VettedAt:              &t3,
		}
		repChange := []nodeevents.Type{nodeevents.UnknownAuditSuspended, nodeevents.OfflineSuspended}
		err = service.UpdateReputation(ctx, node.ID(), "", reputationUpdate, repChange)
		require.NoError(t, err)

		info, err = service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.Equal(t, reputationUpdate.Disqualified, info.Disqualified)
		require.Zero(t, cmp.Diff(info.UnknownAuditSuspended, reputationUpdate.UnknownAuditSuspended, cmpopts.EquateApproxTime(0)))
		require.Zero(t, cmp.Diff(info.OfflineSuspended, reputationUpdate.OfflineSuspended, cmpopts.EquateApproxTime(0)))
		require.Zero(t, cmp.Diff(info.Reputation.Status.VettedAt, reputationUpdate.VettedAt, cmpopts.EquateApproxTime(0)))

		reputationUpdate.Disqualified = &t0
		repChange = []nodeevents.Type{nodeevents.Disqualified}
		err = service.UpdateReputation(ctx, node.ID(), "", reputationUpdate, repChange)
		require.NoError(t, err)

		info, err = service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.Zero(t, cmp.Diff(info.Disqualified, reputationUpdate.Disqualified, cmpopts.EquateApproxTime(0)))

		nodeInfo, err := overlaydb.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              node.ID(),
			ExitInitiatedAt:     t0,
			ExitLoopCompletedAt: t1,
			ExitFinishedAt:      t1,
			ExitSuccess:         true,
		})
		require.NoError(t, err)
		require.NotNil(t, nodeInfo.ExitStatus.ExitFinishedAt)

		// make sure Disqualified field is not updated if a node has finished
		// graceful exit
		reputationUpdate.Disqualified = &t0
		err = service.UpdateReputation(ctx, node.ID(), "", reputationUpdate, repChange)
		require.NoError(t, err)

		exitedNodeInfo, err := service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.Equal(t, info.Disqualified, exitedNodeInfo.Disqualified)
	})
}

func getNodeInfo(nodeID storj.NodeID) overlay.NodeCheckInInfo {
	return overlay.NodeCheckInInfo{
		NodeID: nodeID,
		IsUp:   true,
		Address: &pb.NodeAddress{
			Address: "1.2.3.4",
		},
		Operator: &pb.NodeOperator{
			Email:  "test@email.test",
			Wallet: "0x123",
		},
		Version: &pb.NodeVersion{
			Version:    "v3.0.0",
			CommitHash: "",
			Timestamp:  time.Time{},
			Release:    false,
		},
	}
}

func TestVetAndUnvetNode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.Satellites[0].Overlay.Service
		node := planet.StorageNodes[0]

		// clear existing data
		err := service.TestUnvetNode(ctx, node.ID())
		require.NoError(t, err)
		dossier, err := service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.Nil(t, dossier.Reputation.Status.VettedAt)

		// vet again
		vettedTime, err := service.TestVetNode(ctx, node.ID())
		require.NoError(t, err)
		require.NotNil(t, vettedTime)
		dossier, err = service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.NotNil(t, dossier.Reputation.Status.VettedAt)

		// unvet again
		err = service.TestUnvetNode(ctx, node.ID())
		require.NoError(t, err)
		dossier, err = service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.Nil(t, dossier.Reputation.Status.VettedAt)
	})
}

func TestUpdateReputationNodeEvents(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.SendNodeEmails = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.Satellites[0].Overlay.Service
		node := planet.StorageNodes[0]
		email := "test@storj.test"
		neDB := planet.Satellites[0].DB.NodeEvents()

		now := time.Now()
		repUpdate := overlay.ReputationUpdate{
			UnknownAuditSuspended: &now,
		}

		repChanges := []nodeevents.Type{nodeevents.UnknownAuditSuspended}

		require.NoError(t, service.UpdateReputation(ctx, node.ID(), email, repUpdate, repChanges))

		ne, err := neDB.GetLatestByEmailAndEvent(ctx, email, nodeevents.UnknownAuditSuspended)
		require.NoError(t, err)
		require.Equal(t, email, ne.Email)
		require.Equal(t, node.ID(), ne.NodeID)
		require.Equal(t, nodeevents.UnknownAuditSuspended, ne.Event)

		repUpdate.UnknownAuditSuspended = nil
		repChanges = []nodeevents.Type{nodeevents.UnknownAuditUnsuspended}
		require.NoError(t, service.UpdateReputation(ctx, node.ID(), "test@storj.test", repUpdate, repChanges))

		ne, err = neDB.GetLatestByEmailAndEvent(ctx, email, nodeevents.UnknownAuditUnsuspended)
		require.NoError(t, err)
		require.Equal(t, email, ne.Email)
		require.Equal(t, node.ID(), ne.NodeID)
		require.Equal(t, nodeevents.UnknownAuditUnsuspended, ne.Event)

		repUpdate.OfflineSuspended = &now
		repChanges = []nodeevents.Type{nodeevents.OfflineSuspended}
		require.NoError(t, service.UpdateReputation(ctx, node.ID(), "test@storj.test", repUpdate, repChanges))

		ne, err = neDB.GetLatestByEmailAndEvent(ctx, email, nodeevents.OfflineSuspended)
		require.NoError(t, err)
		require.Equal(t, email, ne.Email)
		require.Equal(t, node.ID(), ne.NodeID)
		require.Equal(t, nodeevents.OfflineSuspended, ne.Event)

		repUpdate.OfflineSuspended = nil
		repChanges = []nodeevents.Type{nodeevents.OfflineUnsuspended}
		require.NoError(t, service.UpdateReputation(ctx, node.ID(), "test@storj.test", repUpdate, repChanges))

		ne, err = neDB.GetLatestByEmailAndEvent(ctx, email, nodeevents.OfflineUnsuspended)
		require.NoError(t, err)
		require.Equal(t, email, ne.Email)
		require.Equal(t, node.ID(), ne.NodeID)
		require.Equal(t, nodeevents.OfflineUnsuspended, ne.Event)

		repUpdate.Disqualified = &now
		repChanges = []nodeevents.Type{nodeevents.Disqualified}
		require.NoError(t, service.UpdateReputation(ctx, node.ID(), "test@storj.test", repUpdate, repChanges))

		ne, err = neDB.GetLatestByEmailAndEvent(ctx, email, nodeevents.Disqualified)
		require.NoError(t, err)
		require.Equal(t, email, ne.Email)
		require.Equal(t, node.ID(), ne.NodeID)
		require.Equal(t, nodeevents.Disqualified, ne.Event)
	})
}

func TestDisqualifyNodeEmails(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.SendNodeEmails = true
				config.Overlay.Node.OnlineWindow = 4 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.Satellites[0].Overlay.Service
		node := planet.StorageNodes[0]
		node.Contact.Chore.Pause(ctx)

		require.NoError(t, service.DisqualifyNode(ctx, node.ID(), overlay.DisqualificationReasonUnknown))

		ne, err := planet.Satellites[0].DB.NodeEvents().GetLatestByEmailAndEvent(ctx, node.Config.Operator.Email, nodeevents.Disqualified)
		require.NoError(t, err)
		require.Equal(t, node.ID(), ne.NodeID)
	})
}

func TestUpdateCheckInNodeEventOnline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.SendNodeEmails = true
				config.Overlay.Node.OnlineWindow = 4 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.Satellites[0].Overlay.Service
		node := planet.StorageNodes[0]
		node.Contact.Chore.Pause(ctx)

		checkInInfo := getNodeInfo(node.ID())
		require.NoError(t, service.UpdateCheckIn(ctx, checkInInfo, time.Now().Add(-24*time.Hour)))
		require.NoError(t, service.UpdateCheckIn(ctx, checkInInfo, time.Now()))

		ne, err := planet.Satellites[0].DB.NodeEvents().GetLatestByEmailAndEvent(ctx, checkInInfo.Operator.Email, nodeevents.Online)
		require.NoError(t, err)
		require.Equal(t, node.ID(), ne.NodeID)
	})
}

func TestUpdateCheckInBelowMinVersionEvent(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.SendNodeEmails = true
				// testplanet storagenode default version is "v0.0.1".
				// set this as minimum version so storagenode doesn't start below it.
				config.Overlay.Node.MinimumVersion = "v0.0.1"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.Satellites[0].Overlay.Service
		node := planet.StorageNodes[0]
		node.Contact.Chore.Pause(ctx)
		email := node.Config.Operator.Email

		getNE := func() nodeevents.NodeEvent {
			ne, err := planet.Satellites[0].DB.NodeEvents().GetLatestByEmailAndEvent(ctx, email, nodeevents.BelowMinVersion)
			require.NoError(t, err)
			require.Equal(t, node.ID(), ne.NodeID)
			require.Equal(t, email, ne.Email)
			require.Equal(t, nodeevents.BelowMinVersion, ne.Event)
			return ne
		}

		nd, err := service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.Nil(t, nd.LastSoftwareUpdateEmail)

		// Set version below minimum
		now := time.Now()
		checkInInfo := getNodeInfo(node.ID())
		checkInInfo.Operator.Email = email

		checkInInfo.Version = &pb.NodeVersion{Version: "v0.0.0"}
		require.NoError(t, service.UpdateCheckIn(ctx, checkInInfo, now))

		nd, err = service.Get(ctx, node.ID())
		require.NoError(t, err)

		lastEmail := nd.LastSoftwareUpdateEmail
		require.NotNil(t, lastEmail)

		// check that software update node event was inserted into nodeevents.DB
		ne0 := getNE()
		require.True(t, ne0.CreatedAt.After(now))

		// check in again and check that another email wasn't sent
		now = now.Add(24 * time.Hour)
		require.NoError(t, service.UpdateCheckIn(ctx, checkInInfo, now))

		nd, err = service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.Equal(t, lastEmail, nd.LastSoftwareUpdateEmail)

		// a node event should not have been inserted, so should be the same as the last node event
		ne1 := getNE()
		require.Equal(t, ne1.CreatedAt, ne0.CreatedAt)

		// check in again after cooldown period has passed and check that email was sent
		require.NoError(t, service.UpdateCheckIn(ctx, checkInInfo, now.Add(planet.Satellites[0].Config.Overlay.NodeSoftwareUpdateEmailCooldown)))

		nd, err = service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.NotNil(t, nd.LastSoftwareUpdateEmail)
		require.True(t, nd.LastSoftwareUpdateEmail.After(*lastEmail))

		ne2 := getNE()
		require.True(t, ne2.CreatedAt.After(ne1.CreatedAt))
	})
}

func TestInsertOfflineNodeEvents(t *testing.T) {
	tagName := "test-tag-name"
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.SendNodeEmails = true
				// testplanet storagenode default version is "v0.0.1".
				// set this as minimum version so storagenode doesn't start below it.
				config.Overlay.Node.MinimumVersion = "v0.0.1"
				config.Overlay.NodeTagsIPPortEmails = []string{tagName}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.Satellites[0].Overlay.Service
		node1 := planet.StorageNodes[0]
		node2 := planet.StorageNodes[1]
		node1.Contact.Chore.Pause(ctx)
		node2.Contact.Chore.Pause(ctx)

		require.NoError(t, service.UpdateNodeTags(ctx, []nodeselection.NodeTag{
			{
				NodeID:   node1.ID(),
				SignedAt: time.Now(),
				Signer:   node1.ID(),
				Name:     tagName,
				Value:    []byte{1, 2, 3},
			},
		}))

		n1LastIPPort := "127.0.0.1:1234"

		for i := 0; i < 2; i++ {
			n := planet.StorageNodes[i]
			lastIPPort := n1LastIPPort
			if i == 1 {
				lastIPPort = ""
			}
			require.NoError(t, planet.Satellites[0].DB.OverlayCache().UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
				NodeID:     n.ID(),
				IsUp:       true,
				LastIPPort: lastIPPort,
				Address:    &pb.NodeAddress{Address: "127.0.0.1"},
				Operator: &pb.NodeOperator{
					Email: n.Config.Operator.Email,
				},
				Version: &pb.NodeVersion{Version: "v1.1.1"},
			}, time.Now().Add(-48*time.Hour), overlay.NodeSelectionConfig{}))
		}

		n, err := service.Get(ctx, node1.ID())
		require.NoError(t, err)
		require.NotNil(t, n)

		count, err := service.InsertOfflineNodeEvents(ctx, 0, 72*time.Hour, 3)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		ne, err := planet.Satellites[0].DB.NodeEvents().GetLatestByEmailAndEvent(ctx, node1.Config.Operator.Email, nodeevents.Offline)
		require.NoError(t, err)
		require.Equal(t, node1.ID(), ne.NodeID)
		require.Equal(t, node1.Config.Operator.Email, ne.Email)
		require.Equal(t, nodeevents.Offline, ne.Event)
		require.NotNil(t, ne.LastIPPort)
		require.Equal(t, n1LastIPPort, *ne.LastIPPort)

		ne, err = planet.Satellites[0].DB.NodeEvents().GetLatestByEmailAndEvent(ctx, node2.Config.Operator.Email, nodeevents.Offline)
		require.NoError(t, err)
		require.Equal(t, node2.ID(), ne.NodeID)
		require.Equal(t, node2.Config.Operator.Email, ne.Email)
		require.Equal(t, nodeevents.Offline, ne.Event)
		require.Nil(t, ne.LastIPPort)
	})
}

func TestAccountingNodeInfo(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		now := time.Now()

		var ids storj.NodeIDList
		var expect []overlay.NodeAccountingInfo
		for i := 0; i < 6; i++ {
			ids = append(ids, storj.NodeID{1, byte(i)})

			var disqualified *time.Time
			if i%2 == 1 {
				disqualified = &now
			}

			expect = append(expect, overlay.NodeAccountingInfo{
				NodeCreationDate: now,
				Wallet:           fmt.Sprintf("0x9b7488BF8b6A4FF21D610e3dd202723f705cD1C%d", i),
				Disqualified:     disqualified,
			})
		}

		for i, id := range ids {
			addr := fmt.Sprintf("127.0.0.%d", i)
			require.NoError(t, cache.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
				NodeID:     id,
				Address:    &pb.NodeAddress{Address: addr},
				LastIPPort: addr + ":9000",
				LastNet:    "127.0.0",
				Version:    &pb.NodeVersion{Version: "v1.0.0"},
				IsUp:       true,
				Operator: &pb.NodeOperator{
					Wallet: expect[i].Wallet,
				},
			}, time.Now(), overlay.NodeSelectionConfig{}))

			if expect[i].Disqualified != nil {
				_, err := cache.DisqualifyNode(ctx, id, *expect[i].Disqualified, overlay.DisqualificationReasonAuditFailure)
				require.NoError(t, err)
			}
		}

		infos, err := cache.AccountingNodeInfo(ctx, ids)
		require.NoError(t, err)

		for i, id := range ids {
			require.Empty(t, cmp.Diff(infos[id], expect[i], cmpopts.EquateApproxTime(15*time.Second)))
		}
	})
}
