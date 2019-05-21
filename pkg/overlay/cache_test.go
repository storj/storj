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

func testCache(ctx context.Context, t *testing.T, store overlay.DB) {
	valid1ID := storj.NodeID{}
	valid2ID := storj.NodeID{}
	missingID := storj.NodeID{}

	_, _ = rand.Read(valid1ID[:])
	_, _ = rand.Read(valid2ID[:])
	_, _ = rand.Read(missingID[:])

	cache := overlay.NewCache(zaptest.NewLogger(t), store, overlay.NodeSelectionConfig{OnlineWindow: time.Hour})

	{ // Put
		err := cache.Put(ctx, valid1ID, pb.Node{Id: valid1ID})
		if err != nil {
			t.Fatal(err)
		}

		err = cache.Put(ctx, valid2ID, pb.Node{Id: valid2ID})
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

func TestUpdateContained(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		cache := planet.Satellites[0].DB.OverlayCache()

		nodeID0 := planet.StorageNodes[0].ID()

		// check that contained default is false
		node0, err := cache.Get(ctx, nodeID0)
		require.NoError(t, err)
		require.False(t, node0.Contained)

		err = cache.UpdateContained(ctx, nodeID0, true)
		require.NoError(t, err)

		node0, err = cache.Get(ctx, nodeID0)
		require.NoError(t, err)
		require.True(t, node0.Contained)

		// sets to true when it's already true
		err = cache.UpdateContained(ctx, nodeID0, true)
		require.NoError(t, err)

		node0, err = cache.Get(ctx, nodeID0)
		require.NoError(t, err)
		require.True(t, node0.Contained)

		nodeID1 := planet.StorageNodes[1].ID()

		// sets to false when it's already false
		err = cache.UpdateContained(ctx, nodeID1, false)
		require.NoError(t, err)

		node1, err := cache.Get(ctx, nodeID1)
		require.NoError(t, err)
		require.False(t, node1.Contained)
	})
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
			_, err = cache.UpdateUptime(ctx, newID, true)
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
