// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
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
		UptimeRatio:       0,
		UptimeCount:       0,
		AuditSuccessRatio: 0,
		AuditCount:        auditCount,
		NewNodePercentage: newNodePercentage,
		OnlineWindow:      time.Hour,
		DistinctIP:        distinctIP,

		ReputationAuditRepairWeight:  1,
		ReputationAuditUplinkWeight:  1,
		ReputationAuditAlpha0:        1,
		ReputationAuditBeta0:         0,
		ReputationAuditLambda:        1,
		ReputationAuditOmega:         1,
		ReputationUptimeRepairWeight: 1,
		ReputationUptimeUplinkWeight: 1,
		ReputationUptimeAlpha0:       1,
		ReputationUptimeBeta0:        0,
		ReputationUptimeLambda:       1,
		ReputationUptimeWeight:       1,
	}
}

func testCache(ctx context.Context, t *testing.T, store overlay.DB) {
	valid1ID := storj.NodeID{}
	valid2ID := storj.NodeID{}
	missingID := storj.NodeID{}
	address := &pb.NodeAddress{Address: "127.0.0.1:0"}

	_, _ = rand.Read(valid1ID[:])
	_, _ = rand.Read(valid2ID[:])
	_, _ = rand.Read(missingID[:])

	cache := overlay.NewCache(zaptest.NewLogger(t), store, testNodeSelectionConfig(0, 0, false))

	{ // Put
		err := cache.Put(ctx, valid1ID, pb.Node{Id: valid1ID, Address: address})
		if err != nil {
			t.Fatal(err)
		}

		err = cache.Put(ctx, valid2ID, pb.Node{Id: valid2ID, Address: address})
		if err != nil {
			t.Fatal(err)
		}
	}

	{ // Get
		_, err := cache.Get(ctx, storj.NodeID{})
		assert.Error(t, err)
		assert.True(t, err == overlay.ErrEmptyNode)

		valid1, err := cache.Get(ctx, valid1ID)
		if assert.NoError(t, err) {
			assert.Equal(t, valid1.Id, valid1ID)
		}

		valid2, err := cache.Get(ctx, valid2ID)
		if assert.NoError(t, err) {
			assert.Equal(t, valid2.Id, valid2ID)
		}

		invalid2, err := cache.Get(ctx, missingID)
		assert.Error(t, err)
		assert.True(t, overlay.ErrNodeNotFound.Has(err))
		assert.Nil(t, invalid2)

		// TODO: add erroring database test
	}

	{ // Paginate

		// should return two nodes
		nodes, more, err := cache.Paginate(ctx, 0, 2)
		assert.NotNil(t, more)
		assert.NoError(t, err)
		assert.Equal(t, len(nodes), 2)

		// should return no nodes
		zero, more, err := cache.Paginate(ctx, 0, 0)
		assert.NoError(t, err)
		assert.NotNil(t, more)
		assert.NotEqual(t, len(zero), 0)
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

		// put nodes in cache
		for i := 0; i < totalNodes; i++ {
			newID := storj.NodeID{}
			_, _ = rand.Read(newID[:])
			err := cache.UpdateAddress(ctx, &pb.Node{Id: newID})
			require.NoError(t, err)
			_, err = cache.UpdateNodeInfo(ctx, newID, &pb.InfoResponse{
				Type:     pb.NodeType_STORAGE,
				Capacity: &pb.NodeCapacity{},
			})
			require.NoError(t, err)
			_, err = cache.UpdateUptime(ctx, newID, true, 1, 0, 1, 1)
			require.NoError(t, err)
			allIDs[i] = newID
			nodeCounts[newID] = 0
		}

		// select numNodesToSelect nodes selectIterations times
		for i := 0; i < selectIterations; i++ {
			var nodes []*pb.Node
			var err error

			if i%2 == 0 {
				nodes, err = cache.SelectStorageNodes(ctx, numNodesToSelect, &overlay.NodeCriteria{OnlineWindow: time.Hour})
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

func TestIsVetted(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.AuditCount = 1
				config.Overlay.Node.AuditSuccessRatio = 1
				config.Overlay.Node.UptimeCount = 1
				config.Overlay.Node.UptimeRatio = 1
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var err error
		satellite := planet.Satellites[0]
		service := satellite.Overlay.Service

		_, err = satellite.DB.OverlayCache().UpdateStats(ctx, &overlay.UpdateRequest{
			NodeID:       planet.StorageNodes[0].ID(),
			IsUp:         true,
			AuditSuccess: true,
		})
		assert.NoError(t, err)

		reputable, err := service.IsVetted(ctx, planet.StorageNodes[0].ID())
		require.NoError(t, err)
		assert.True(t, reputable)

		reputable, err = service.IsVetted(ctx, planet.StorageNodes[1].ID())
		require.NoError(t, err)
		assert.False(t, reputable)
	})
}

func TestNodeInfo(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.StorageNodes[0].Storage2.Monitor.Loop.Pause()
		planet.Satellites[0].Discovery.Service.Refresh.Pause()

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
