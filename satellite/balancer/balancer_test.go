// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package balancer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testredis"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/taskqueue"
)

func TestJobSerialization(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redisServer, err := testredis.Start(ctx)
	require.NoError(t, err)
	defer ctx.Check(redisServer.Close)

	client, err := taskqueue.NewClient(ctx, taskqueue.Config{
		Address:  "redis://" + redisServer.Addr(),
		Group:    "test-group",
		Consumer: "test-consumer",
	})
	require.NoError(t, err)
	defer ctx.Check(client.Close)

	expected := Job{
		StreamID:   testrand.UUID(),
		Position:   12345678,
		SourceNode: testrand.NodeID(),
		DestNode:   testrand.NodeID(),
	}

	err = client.Push(ctx, "balancer", expected)
	require.NoError(t, err)

	var got Job
	ok, err := client.Pop(ctx, "balancer", &got, time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, expected.StreamID, got.StreamID)
	assert.Equal(t, expected.Position, got.Position)
	assert.Equal(t, expected.SourceNode, got.SourceNode)
	assert.Equal(t, expected.DestNode, got.DestNode)
}

func TestProcessSegment(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redisServer, err := testredis.Start(ctx)
	require.NoError(t, err)
	defer ctx.Check(redisServer.Close)

	client, err := taskqueue.NewClient(ctx, taskqueue.Config{
		Address:  "redis://" + redisServer.Addr(),
		Group:    "test-group",
		Consumer: "test-consumer",
	})
	require.NoError(t, err)
	defer ctx.Check(client.Close)

	// Create nodes:
	//   Group "A": node1 (FreeDisk=100, overfull), node2 (FreeDisk=300, underfull) => avg=200
	//   Group "B": node3 (FreeDisk=150), node4 (FreeDisk=250) => avg=200
	//   Group "A": node5 (FreeDisk=400, underfull, not in segment) => candidate destination
	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	node3 := testrand.NodeID()
	node4 := testrand.NodeID()
	node5 := testrand.NodeID()

	nodeCache := map[storj.NodeID]*nodeInfo{
		node1: {node: nodeselection.SelectedNode{ID: node1, FreeDisk: 100, LastNet: "A"}, group: "A", expectedFree: 200, currentFree: 100},
		node2: {node: nodeselection.SelectedNode{ID: node2, FreeDisk: 300, LastNet: "A"}, group: "A", expectedFree: 200, currentFree: 300},
		node3: {node: nodeselection.SelectedNode{ID: node3, FreeDisk: 150, LastNet: "B"}, group: "B", expectedFree: 200, currentFree: 150},
		node4: {node: nodeselection.SelectedNode{ID: node4, FreeDisk: 250, LastNet: "B"}, group: "B", expectedFree: 200, currentFree: 250},
		node5: {node: nodeselection.SelectedNode{ID: node5, FreeDisk: 400, LastNet: "A"}, group: "A", expectedFree: 200, currentFree: 400},
	}

	// Pre-build destination candidates for group A: node5 (surplus=200), node2 (surplus=100).
	groupDestCandidates := map[string][]*nodeInfo{
		"A": {nodeCache[node5], nodeCache[node2]},
		"B": {nodeCache[node4]},
	}

	placements := nodeselection.PlacementDefinitions{
		storj.DefaultPlacement: {
			ID:        storj.DefaultPlacement,
			Invariant: nodeselection.AllGood(),
		},
	}

	observer := &Balancer{
		log:                 zaptest.NewLogger(t),
		config:              Config{StreamID: "balancer"},
		client:              client,
		placements:          placements,
		nodeCache:           nodeCache,
		groupDestCandidates: groupDestCandidates,
	}

	// Segment with pieces on node1 (group A, overfull) and node3 (group B, overfull).
	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: node1},
			{Number: 1, StorageNode: node3},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &balancerFork{observer: observer}
	err = fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	// Flush remaining jobs.
	if len(fork.jobs) > 0 {
		err = client.PushBatch(ctx, "balancer", fork.jobs)
		require.NoError(t, err)
	}

	// Should have produced a job: source=node1 (biggest overfull, diff=100), dest=node5 (biggest surplus in group A).
	var job Job
	ok, err := client.Pop(ctx, "balancer", &job, time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, segment.StreamID, job.StreamID)
	assert.Equal(t, node1, job.SourceNode)
	assert.Equal(t, node5, job.DestNode)

	// Verify currentFree was adjusted after the move.
	pieceSize := segment.PieceSize()
	assert.Equal(t, int64(100)+pieceSize, nodeCache[node1].currentFree, "source should gain free space")
	assert.Equal(t, int64(400)-pieceSize, nodeCache[node5].currentFree, "dest should lose free space")
}

func TestInvariantPreventsMove(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redisServer, err := testredis.Start(ctx)
	require.NoError(t, err)
	defer ctx.Check(redisServer.Close)

	client, err := taskqueue.NewClient(ctx, taskqueue.Config{
		Address:  "redis://" + redisServer.Addr(),
		Group:    "test-group",
		Consumer: "test-consumer",
	})
	require.NoError(t, err)
	defer ctx.Check(client.Close)

	// Use different subnets where the swap creates NEW violations.
	// Invariant: max 1 piece per last_net.
	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	destNode := testrand.NodeID()

	nodeCache := map[storj.NodeID]*nodeInfo{
		node1:    {node: nodeselection.SelectedNode{ID: node1, FreeDisk: 100, LastNet: "1.0.0.0"}, group: "A", expectedFree: 300, currentFree: 100},
		node2:    {node: nodeselection.SelectedNode{ID: node2, FreeDisk: 200, LastNet: "2.0.0.0"}, group: "A", expectedFree: 300, currentFree: 200},
		destNode: {node: nodeselection.SelectedNode{ID: destNode, FreeDisk: 500, LastNet: "2.0.0.0"}, group: "A", expectedFree: 300, currentFree: 500},
	}

	groupDestCandidates := map[string][]*nodeInfo{
		"A": {nodeCache[destNode]},
	}

	placements := nodeselection.PlacementDefinitions{
		storj.DefaultPlacement: {
			ID:        storj.DefaultPlacement,
			Invariant: nodeselection.ClumpingByAttribute(nodeselection.LastNetAttribute, 1),
		},
	}

	observer := &Balancer{
		log:                 zaptest.NewLogger(t),
		config:              Config{StreamID: "balancer"},
		client:              client,
		placements:          placements,
		nodeCache:           nodeCache,
		groupDestCandidates: groupDestCandidates,
	}

	// Segment: piece0 on node1 (subnet 1.0.0.0), piece1 on node2 (subnet 2.0.0.0).
	// Original: 0 violations (each subnet has 1 piece).
	// If we swap node1→destNode (subnet 2.0.0.0): piece0 on 2.0.0.0, piece1 on 2.0.0.0 = 1 violation.
	// newCount (1) > origCount (0) → move rejected.
	segment := rangedloop.Segment{
		StreamID:      testrand.UUID(),
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: node1},
			{Number: 1, StorageNode: node2},
		},
	}

	fork := &balancerFork{observer: observer}
	err = fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	// No job should be produced since the invariant prevents the move.
	assert.Empty(t, fork.jobs)

	// Also verify nothing in Redis.
	var job Job
	ok, err := client.Pop(ctx, "balancer", &job, 100*time.Millisecond)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestNoDestinationCandidates(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redisServer, err := testredis.Start(ctx)
	require.NoError(t, err)
	defer ctx.Check(redisServer.Close)

	client, err := taskqueue.NewClient(ctx, taskqueue.Config{
		Address:  "redis://" + redisServer.Addr(),
		Group:    "test-group",
		Consumer: "test-consumer",
	})
	require.NoError(t, err)
	defer ctx.Check(client.Close)

	// All nodes in group "A" are in the segment, so no destination candidate is available.
	node1 := testrand.NodeID()
	node2 := testrand.NodeID()

	nodeCache := map[storj.NodeID]*nodeInfo{
		node1: {node: nodeselection.SelectedNode{ID: node1, FreeDisk: 100}, group: "A", expectedFree: 200, currentFree: 100},
		node2: {node: nodeselection.SelectedNode{ID: node2, FreeDisk: 300}, group: "A", expectedFree: 200, currentFree: 300},
	}

	// node2 is underfull, but it's already in the segment.
	groupDestCandidates := map[string][]*nodeInfo{
		"A": {nodeCache[node2]},
	}

	placements := nodeselection.PlacementDefinitions{
		storj.DefaultPlacement: {
			ID:        storj.DefaultPlacement,
			Invariant: nodeselection.AllGood(),
		},
	}

	observer := &Balancer{
		log:                 zaptest.NewLogger(t),
		config:              Config{StreamID: "balancer"},
		client:              client,
		placements:          placements,
		nodeCache:           nodeCache,
		groupDestCandidates: groupDestCandidates,
	}

	segment := rangedloop.Segment{
		StreamID:      testrand.UUID(),
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: node1},
			{Number: 1, StorageNode: node2},
		},
	}

	fork := &balancerFork{observer: observer}
	err = fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	// No job produced since node2 (only dest candidate) is already in the segment.
	assert.Empty(t, fork.jobs)
}
