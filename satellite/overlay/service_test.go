// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storagenode"
)

func TestCache_Database(t *testing.T) {
	t.Parallel()

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		testCache(ctx, t, db.OverlayCache())
	})
}

// returns a NodeSelectionConfig with sensible test values
func testNodeSelectionConfig(auditCount int64, newNodePercentage float64, distinctIP bool) overlay.NodeSelectionConfig {
	return overlay.NodeSelectionConfig{
		UptimeCount:       0,
		AuditCount:        auditCount,
		NewNodePercentage: newNodePercentage,
		OnlineWindow:      time.Hour,
		DistinctIP:        distinctIP,

		AuditReputationRepairWeight:  1,
		AuditReputationUplinkWeight:  1,
		AuditReputationAlpha0:        1,
		AuditReputationBeta0:         0,
		AuditReputationLambda:        1,
		AuditReputationWeight:        1,
		AuditReputationDQ:            0.5,
		UptimeReputationRepairWeight: 1,
		UptimeReputationUplinkWeight: 1,
		UptimeReputationAlpha0:       1,
		UptimeReputationBeta0:        0,
		UptimeReputationLambda:       1,
		UptimeReputationWeight:       1,
		UptimeReputationDQ:           0.5,
	}
}

func testCache(ctx context.Context, t *testing.T, store overlay.DB) {
	valid1ID := testrand.NodeID()
	valid2ID := testrand.NodeID()
	valid3ID := testrand.NodeID()
	missingID := testrand.NodeID()
	address := &pb.NodeAddress{Address: "127.0.0.1:0"}

	nodeSelectionConfig := testNodeSelectionConfig(0, 0, false)
	serviceConfig := overlay.Config{Node: nodeSelectionConfig, UpdateStatsBatchSize: 100}
	service := overlay.NewService(zaptest.NewLogger(t), store, serviceConfig)

	{ // Put
		err := service.Put(ctx, valid1ID, pb.Node{Id: valid1ID, Address: address})
		require.NoError(t, err)

		err = service.Put(ctx, valid2ID, pb.Node{Id: valid2ID, Address: address})
		require.NoError(t, err)

		err = service.Put(ctx, valid3ID, pb.Node{Id: valid3ID, Address: address})
		require.NoError(t, err)

		_, err = service.UpdateUptime(ctx, valid3ID, false)
		require.NoError(t, err)
	}

	{ // Get
		_, err := service.Get(ctx, storj.NodeID{})
		require.Error(t, err)
		require.True(t, err == overlay.ErrEmptyNode)

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

	{ // Paginate

		// should return two nodes
		nodes, more, err := service.Paginate(ctx, 0, 2)
		assert.NotNil(t, more)
		assert.NoError(t, err)
		assert.Equal(t, len(nodes), 2)

		// should return no nodes
		zero, more, err := service.Paginate(ctx, 0, 0)
		assert.NoError(t, err)
		assert.NotNil(t, more)
		assert.NotEqual(t, len(zero), 0)
	}

	{ // PaginateQualified

		// should return two nodes
		nodes, more, err := service.PaginateQualified(ctx, 0, 3)
		assert.NotNil(t, more)
		assert.NoError(t, err)
		assert.Equal(t, len(nodes), 2)
	}

	{ // Reputation
		valid1, err := service.Get(ctx, valid1ID)
		require.NoError(t, err)
		require.EqualValues(t, valid1.Id, valid1ID)
		require.EqualValues(t, valid1.Reputation.AuditReputationAlpha, nodeSelectionConfig.AuditReputationAlpha0)
		require.EqualValues(t, valid1.Reputation.AuditReputationBeta, nodeSelectionConfig.AuditReputationBeta0)
		require.EqualValues(t, valid1.Reputation.UptimeReputationAlpha, nodeSelectionConfig.UptimeReputationAlpha0)
		require.EqualValues(t, valid1.Reputation.UptimeReputationBeta, nodeSelectionConfig.UptimeReputationBeta0)
		require.Nil(t, valid1.Reputation.Disqualified)

		stats, err := service.UpdateStats(ctx, &overlay.UpdateRequest{
			NodeID:       valid1ID,
			IsUp:         true,
			AuditSuccess: false,
		})
		require.NoError(t, err)
		newAuditAlpha := 1
		newAuditBeta := 1
		newUptimeAlpha := 2
		newUptimeBeta := 0
		require.EqualValues(t, stats.AuditReputationAlpha, newAuditAlpha)
		require.EqualValues(t, stats.AuditReputationBeta, newAuditBeta)
		require.EqualValues(t, stats.UptimeReputationAlpha, newUptimeAlpha)
		require.EqualValues(t, stats.UptimeReputationBeta, newUptimeBeta)
		require.NotNil(t, stats.Disqualified)
		require.True(t, time.Now().UTC().Sub(*stats.Disqualified) < time.Minute)

		stats, err = service.UpdateUptime(ctx, valid2ID, false)
		require.NoError(t, err)
		newUptimeAlpha = 1
		newUptimeBeta = 1
		require.EqualValues(t, stats.AuditReputationAlpha, nodeSelectionConfig.AuditReputationAlpha0)
		require.EqualValues(t, stats.AuditReputationBeta, nodeSelectionConfig.AuditReputationBeta0)
		require.EqualValues(t, stats.UptimeReputationAlpha, newUptimeAlpha)
		require.EqualValues(t, stats.UptimeReputationBeta, newUptimeBeta)
		require.NotNil(t, stats.Disqualified)
		require.True(t, time.Now().UTC().Sub(*stats.Disqualified) < time.Minute)
		dqTime := *stats.Disqualified

		// should not update once already disqualified
		_, err = service.BatchUpdateStats(ctx, []*overlay.UpdateRequest{{
			NodeID:       valid2ID,
			IsUp:         false,
			AuditSuccess: true,
		}})
		require.NoError(t, err)
		dossier, err := service.Get(ctx, valid2ID)

		require.NoError(t, err)
		require.EqualValues(t, dossier.Reputation.AuditReputationAlpha, nodeSelectionConfig.AuditReputationAlpha0)
		require.EqualValues(t, dossier.Reputation.AuditReputationBeta, nodeSelectionConfig.AuditReputationBeta0)
		require.EqualValues(t, dossier.Reputation.UptimeReputationAlpha, newUptimeAlpha)
		require.EqualValues(t, dossier.Reputation.UptimeReputationBeta, newUptimeBeta)
		require.NotNil(t, dossier.Disqualified)
		require.Equal(t, *dossier.Disqualified, dqTime)

	}
}

func TestRandomizedSelection(t *testing.T) {
	t.Parallel()

	totalNodes := 1000
	selectIterations := 100
	numNodesToSelect := 100
	minSelectCount := 3 // TODO: compute this limit better

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		cache := db.OverlayCache()
		allIDs := make(storj.NodeIDList, totalNodes)
		nodeCounts := make(map[storj.NodeID]int)
		defaults := overlay.NodeSelectionConfig{
			AuditReputationAlpha0:  1,
			AuditReputationBeta0:   0,
			UptimeReputationAlpha0: 1,
			UptimeReputationBeta0:  0,
		}

		// put nodes in cache
		for i := 0; i < totalNodes; i++ {
			newID := testrand.NodeID()

			err := cache.UpdateAddress(ctx, &pb.Node{Id: newID}, defaults)
			require.NoError(t, err)
			_, err = cache.UpdateNodeInfo(ctx, newID, &pb.InfoResponse{
				Type:     pb.NodeType_STORAGE,
				Capacity: &pb.NodeCapacity{},
			})
			require.NoError(t, err)

			if i%2 == 0 { // make half of nodes "new" and half "vetted"
				_, err = cache.UpdateStats(ctx, &overlay.UpdateRequest{
					NodeID:       newID,
					IsUp:         true,
					AuditSuccess: true,
					AuditLambda:  1,
					AuditWeight:  1,
					AuditDQ:      0.5,
					UptimeLambda: 1,
					UptimeWeight: 1,
					UptimeDQ:     0.5,
				})
				require.NoError(t, err)
			}

			allIDs[i] = newID
			nodeCounts[newID] = 0
		}

		// select numNodesToSelect nodes selectIterations times
		for i := 0; i < selectIterations; i++ {
			var nodes []*pb.Node
			var err error

			if i%2 == 0 {
				nodes, err = cache.SelectStorageNodes(ctx, numNodesToSelect, &overlay.NodeCriteria{
					OnlineWindow: time.Hour,
					AuditCount:   1,
				})
				require.NoError(t, err)
			} else {
				nodes, err = cache.SelectNewStorageNodes(ctx, numNodesToSelect, &overlay.NodeCriteria{
					OnlineWindow: time.Hour,
					AuditCount:   1,
				})
				require.NoError(t, err)
			}
			require.Len(t, nodes, numNodesToSelect)

			for _, node := range nodes {
				nodeCounts[node.Id]++
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

		assert.Equal(t, pb.NodeType_STORAGE, node.Type)
		assert.NotEmpty(t, node.Operator.Email)
		assert.NotEmpty(t, node.Operator.Wallet)
		assert.Equal(t, planet.StorageNodes[0].Local().Operator, node.Operator)
		assert.NotEmpty(t, node.Capacity.FreeBandwidth)
		assert.NotEmpty(t, node.Capacity.FreeDisk)
		assert.Equal(t, planet.StorageNodes[0].Local().Capacity, node.Capacity)
		assert.NotEmpty(t, node.Version.Version)
		assert.Equal(t, planet.StorageNodes[0].Local().Version.Version, node.Version.Version)
	})
}

func TestKnownReliable(t *testing.T) {
	onlineWindow := 500 * time.Millisecond

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.OnlineWindow = onlineWindow
			},
			StorageNode: func(index int, config *storagenode.Config) {
				config.Contact.Interval = onlineWindow / 2
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		service := satellite.Overlay.Service

		// Disqualify storage node #0
		stats, err := service.UpdateStats(ctx, &overlay.UpdateRequest{
			NodeID:       planet.StorageNodes[0].ID(),
			AuditSuccess: false,
		})
		require.NoError(t, err)
		require.NotNil(t, stats.Disqualified)

		// Stop storage node #1
		err = planet.StopPeer(planet.StorageNodes[1])
		require.NoError(t, err)
		_, err = service.UpdateUptime(ctx, planet.StorageNodes[1].ID(), false)
		require.NoError(t, err)

		// Sleep for the duration of the online window and check that storage node #1 is offline
		time.Sleep(onlineWindow)
		node, err := service.Get(ctx, planet.StorageNodes[1].ID())
		require.NoError(t, err)
		require.False(t, service.IsOnline(node))

		// Check that only storage nodes #2 and #3 are reliable
		result, err := service.KnownReliable(ctx, []storj.NodeID{
			planet.StorageNodes[0].ID(),
			planet.StorageNodes[1].ID(),
			planet.StorageNodes[2].ID(),
			planet.StorageNodes[3].ID(),
		})
		require.NoError(t, err)
		require.Len(t, result, 2)

		// Sort the storage nodes for predictable checks
		expectedReliable := []pb.Node{planet.StorageNodes[2].Local().Node, planet.StorageNodes[3].Local().Node}
		sort.Slice(expectedReliable, func(i, j int) bool { return expectedReliable[i].Id.Less(expectedReliable[j].Id) })
		sort.Slice(result, func(i, j int) bool { return result[i].Id.Less(result[j].Id) })

		// Assert the reliable nodes are the expected ones
		for i, node := range result {
			assert.Equal(t, expectedReliable[i].Id, node.Id)
			assert.Equal(t, expectedReliable[i].Address, node.Address)
			assert.NotNil(t, node.LastIp)
		}
	})
}

func TestUpdateCheckIn(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		// setup
		nodeID := storj.NodeID{1, 2, 3}
		expectedEmail := "test@email.com"
		expectedAddress := "1.2.4.4"
		info := overlay.NodeCheckInInfo{
			NodeID: nodeID,
			Address: &pb.NodeAddress{
				Address: expectedAddress,
			},
			IsUp: true,
			Capacity: &pb.NodeCapacity{
				FreeBandwidth: int64(1234),
				FreeDisk:      int64(5678),
			},
			Operator: &pb.NodeOperator{
				Email:  expectedEmail,
				Wallet: "0x123",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
		}
		expectedNode := &overlay.NodeDossier{
			Node: pb.Node{
				Id:     nodeID,
				LastIp: info.LastIP,
				Address: &pb.NodeAddress{
					Address:   info.Address.GetAddress(),
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
				},
			},
			Type: pb.NodeType_STORAGE,
			Operator: pb.NodeOperator{
				Email:  info.Operator.GetEmail(),
				Wallet: info.Operator.GetWallet(),
			},
			Capacity: pb.NodeCapacity{
				FreeBandwidth: info.Capacity.GetFreeBandwidth(),
				FreeDisk:      info.Capacity.GetFreeDisk(),
			},
			Reputation: overlay.NodeStats{
				UptimeCount:           1,
				UptimeSuccessCount:    1,
				UptimeReputationAlpha: 1,
			},
			Version: pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			Contained:    false,
			Disqualified: nil,
			PieceCount:   0,
			ExitStatus:   overlay.ExitStatus{NodeID: nodeID},
		}
		config := overlay.NodeSelectionConfig{
			UptimeReputationLambda: 0.99,
			UptimeReputationWeight: 1.0,
			UptimeReputationDQ:     0,
		}

		// confirm the node doesn't exist in nodes table yet
		_, err := db.OverlayCache().Get(ctx, nodeID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "node not found")

		// check-in for that node id, which should add the node
		// to the nodes tables in the database
		startOfTest := time.Now().UTC()
		err = db.OverlayCache().UpdateCheckIn(ctx, info, time.Now().UTC(), config)
		require.NoError(t, err)

		// confirm that the node is now in the nodes table with the
		// correct fields set
		actualNode, err := db.OverlayCache().Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, actualNode.Reputation.LastContactSuccess.After(startOfTest))
		require.True(t, actualNode.Reputation.LastContactFailure.UTC().Equal(time.Time{}.UTC()))

		// we need to overwrite the times so that the deep equal considers them the same
		expectedNode.Reputation.LastContactSuccess = actualNode.Reputation.LastContactSuccess
		expectedNode.Reputation.LastContactFailure = actualNode.Reputation.LastContactFailure
		expectedNode.Version.Timestamp = actualNode.Version.Timestamp
		expectedNode.CreatedAt = actualNode.CreatedAt
		require.Equal(t, expectedNode, actualNode)

		// confirm that we can update the address field
		startOfUpdateTest := time.Now().UTC()
		expectedAddress = "9.8.7.6"
		updatedInfo := overlay.NodeCheckInInfo{
			NodeID: nodeID,
			Address: &pb.NodeAddress{
				Address: expectedAddress,
			},
			IsUp: true,
			Capacity: &pb.NodeCapacity{
				FreeBandwidth: int64(12355),
			},
			Version: &pb.NodeVersion{
				Version:    "v0.1.0",
				CommitHash: "abc123",
				Timestamp:  time.Now().UTC(),
				Release:    true,
			},
		}
		// confirm that the updated node is in the nodes table with the
		// correct updated fields set
		err = db.OverlayCache().UpdateCheckIn(ctx, updatedInfo, time.Now().UTC(), config)
		require.NoError(t, err)
		updatedNode, err := db.OverlayCache().Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, updatedNode.Reputation.LastContactSuccess.After(startOfUpdateTest))
		require.True(t, updatedNode.Reputation.LastContactFailure.Equal(time.Time{}.UTC()))
		require.Equal(t, updatedNode.Address.GetAddress(), expectedAddress)
		require.Equal(t, updatedNode.Reputation.UptimeSuccessCount, actualNode.Reputation.UptimeSuccessCount+1)
		require.Equal(t, updatedNode.Capacity.GetFreeBandwidth(), int64(12355))
		require.Equal(t, updatedInfo.Version.GetVersion(), updatedNode.Version.GetVersion())
		require.Equal(t, updatedInfo.Version.GetCommitHash(), updatedNode.Version.GetCommitHash())
		require.Equal(t, updatedInfo.Version.GetRelease(), updatedNode.Version.GetRelease())
		require.True(t, updatedNode.Version.GetTimestamp().After(info.Version.GetTimestamp()))

		// confirm we can udpate IsUp field
		startOfUpdateTest2 := time.Now().UTC()
		updatedInfo2 := overlay.NodeCheckInInfo{
			NodeID: nodeID,
			Address: &pb.NodeAddress{
				Address: "9.8.7.6",
			},
			IsUp: false,
			Capacity: &pb.NodeCapacity{
				FreeBandwidth: int64(12355),
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
		}
		err = db.OverlayCache().UpdateCheckIn(ctx, updatedInfo2, time.Now().UTC(), config)
		require.NoError(t, err)
		updated2Node, err := db.OverlayCache().Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, updated2Node.Reputation.LastContactSuccess.Equal(updatedNode.Reputation.LastContactSuccess))
		require.Equal(t, updated2Node.Reputation.UptimeSuccessCount, updatedNode.Reputation.UptimeSuccessCount)
		require.True(t, updated2Node.Reputation.LastContactFailure.After(startOfUpdateTest2))
	})
}
