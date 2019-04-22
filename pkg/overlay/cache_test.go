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

	cache := overlay.NewCache(zaptest.NewLogger(t), store, overlay.NodeSelectionConfig{
		OnlineWindow:      time.Hour,
		AuditSuccessRatio: 0.5,
		UptimeRatio:       0.5,
	})

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
		nodes, err := cache.GetAll(ctx, storj.NodeIDList{valid2ID, valid1ID, valid2ID}, nil)
		assert.NoError(t, err)
		assert.Equal(t, nodes[0].Id, valid2ID)
		assert.Equal(t, nodes[1].Id, valid1ID)
		assert.Equal(t, nodes[2].Id, valid2ID)

		nodes, err = cache.GetAll(ctx, storj.NodeIDList{valid1ID, missingID}, nil)
		assert.NoError(t, err)
		assert.Equal(t, nodes[0].Id, valid1ID)
		assert.Nil(t, nodes[1])

		nodes, err = cache.GetAll(ctx, make(storj.NodeIDList, 2), nil)
		assert.NoError(t, err)
		assert.Nil(t, nodes[0])
		assert.Nil(t, nodes[1])

		_, err = cache.GetAll(ctx, storj.NodeIDList{}, nil)
		assert.True(t, overlay.OverlayError.Has(err))

		// TODO: add erroring database test
	}

	{ // InvalidNodes
		for _, tt := range []struct {
			nodeID             storj.NodeID
			auditSuccessCount  int64
			auditCount         int64
			auditSuccessRatio  float64
			uptimeSuccessCount int64
			uptimeCount        int64
			uptimeRatio        float64
		}{
			{storj.NodeID{1}, 20, 20, 1, 20, 20, 1},   // good audit success
			{storj.NodeID{2}, 5, 20, 0.25, 20, 20, 1}, // bad audit success, good uptime
			{storj.NodeID{3}, 20, 20, 1, 5, 20, 0.25}, // good audit success, bad uptime
			{storj.NodeID{4}, 0, 0, 0, 20, 20, 1},     // "bad" audit success, no audits
			{storj.NodeID{5}, 20, 20, 1, 0, 0, 0.25},  // "bad" uptime success, no checks
			{storj.NodeID{6}, 0, 1, 0, 5, 5, 1},       // bad audit success exactly one audit
			{storj.NodeID{7}, 0, 20, 0, 20, 20, 1},    // bad ratios, excluded from query
		} {
			err := cache.Put(ctx, tt.nodeID, pb.Node{Id: tt.nodeID})
			require.NoError(t, err)

			as, u, a := int64(0), int64(0), int64(0)
			for i := int64(0); i < tt.auditSuccessCount; i++ {
				var audit, auditSuccess, isUp bool
				if as > tt.auditSuccessCount {
					auditSuccess = true
					as++
				}
				if u > tt.uptimeSuccessCount {
					isUp = true
					u++
				}
				if a > tt.auditCount {
					isUp = true
					a++
				}
				if audit {
					_, err = cache.UpdateStats(ctx, &overlay.UpdateRequest{NodeID: tt.nodeID, AuditSuccess: auditSuccess, IsUp: isUp})
					require.NoError(t, err)
				} else {
					_, err = cache.UpdateUptime(ctx, tt.nodeID, isUp)
					require.NoError(t, err)
				}
			}
		}

		nodeIds := storj.NodeIDList{storj.NodeID{1}, storj.NodeID{2}, storj.NodeID{3}, storj.NodeID{4}, storj.NodeID{5}, storj.NodeID{6}}
		invalidNodes, err := cache.GetAll(ctx, nodeIds, func(n *overlay.NodeDossier) bool { return !cache.IsValid(n) })
		require.NoError(t, err)
		invalid := []storj.NodeID{}
		for _, x := range invalidNodes {
			invalid = append(invalid, x.Node.Id)
		}

		assert.Contains(t, invalid, storj.NodeID{2})
		assert.Contains(t, invalid, storj.NodeID{3})
		assert.Contains(t, invalid, storj.NodeID{6})
		assert.Len(t, invalid, 3)
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

			if reputable {
				nodes, err = cache.SelectStorageNodes(ctx, numNodesToSelect, &overlay.NodeCriteria{
					OnlineWindow: time.Hour,
				})
				require.NoError(t, err)
			} else {
				nodes, err = cache.SelectNewStorageNodes(ctx, numNodesToSelect, &overlay.NewNodeCriteria{
					OnlineWindow:   time.Hour,
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
