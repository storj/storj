// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
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

	cache := overlay.NewCache(zaptest.NewLogger(t), store, overlay.NodeSelectionConfig{})

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

	{ // GetAll
		nodes, err := cache.GetAll(ctx, storj.NodeIDList{valid2ID, valid1ID, valid2ID})
		assert.NoError(t, err)
		assert.Equal(t, nodes[0].Id, valid2ID)
		assert.Equal(t, nodes[1].Id, valid1ID)
		assert.Equal(t, nodes[2].Id, valid2ID)

		nodes, err = cache.GetAll(ctx, storj.NodeIDList{valid1ID, missingID})
		assert.NoError(t, err)
		assert.Equal(t, nodes[0].Id, valid1ID)
		assert.Nil(t, nodes[1])

		nodes, err = cache.GetAll(ctx, make(storj.NodeIDList, 2))
		assert.NoError(t, err)
		assert.Nil(t, nodes[0])
		assert.Nil(t, nodes[1])

		_, err = cache.GetAll(ctx, storj.NodeIDList{})
		assert.True(t, overlay.OverlayError.Has(err))

		// TODO: add erroring database test
	}

	{ // List
		list, err := cache.List(ctx, storj.NodeID{}, 3)
		assert.NoError(t, err)
		assert.NotNil(t, list)
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

	{ // Delete
		// Test standard delete
		err := cache.Delete(ctx, valid1ID)
		assert.NoError(t, err)

		// Check that it was deleted
		deleted, err := cache.Get(ctx, valid1ID)
		assert.Error(t, err)
		assert.Nil(t, deleted)
		assert.True(t, overlay.ErrNodeNotFound.Has(err))

		// Test idempotent delete / non existent key delete
		err = cache.Delete(ctx, valid1ID)
		assert.NoError(t, err)

		// Test empty key delete
		err = cache.Delete(ctx, storj.NodeID{})
		assert.Error(t, err)
		assert.True(t, err == overlay.ErrEmptyNode)
	}
}

func TestRandomizedSelection(t *testing.T) {
	t.Parallel()
	testRandomizedSelection(t, true)
	testRandomizedSelection(t, false)
}

func testRandomizedSelection(t *testing.T, reputable bool) {
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
			err := cache.Update(ctx, &pb.Node{
				Id:           newID,
				Type:         pb.NodeType_STORAGE,
				Restrictions: &pb.NodeRestrictions{},
				Reputation:   &pb.NodeStats{},
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

			if reputable {
				nodes, err = cache.SelectStorageNodes(ctx, numNodesToSelect, &overlay.NodeCriteria{})
				require.NoError(t, err)
			} else {
				nodes, err = cache.SelectNewStorageNodes(ctx, numNodesToSelect, &overlay.NewNodeCriteria{
					AuditThreshold: 1,
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
