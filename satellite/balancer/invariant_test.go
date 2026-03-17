// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package balancer

import (
	"context"
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
	"storj.io/storj/shared/location"
)

func newTestInvariantObserver(t *testing.T, ctx *testcontext.Context) (*Invariant, *taskqueue.Client, func()) {
	redisServer, err := testredis.Start(ctx)
	require.NoError(t, err)

	client, err := taskqueue.NewClient(ctx, taskqueue.Config{
		Address:  "redis://" + redisServer.Addr(),
		Group:    "test-group",
		Consumer: "test-consumer",
	})
	require.NoError(t, err)

	fixer := &Invariant{
		log:    zaptest.NewLogger(t),
		config: InvariantConfig{StreamID: "invariant"},
		client: client,
	}

	cleanup := func() {
		require.NoError(t, client.Close())
		require.NoError(t, redisServer.Close())
	}

	return fixer, client, cleanup
}

func TestInvariantClumpingViolation(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fixer, client, cleanup := newTestInvariantObserver(t, ctx)
	defer cleanup()

	// Two nodes on the same subnet, one on a different subnet.
	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	destNode := testrand.NodeID()

	fixer.nodeMap = map[storj.NodeID]*nodeselection.SelectedNode{
		node1:    {ID: node1, LastNet: "subnet-A"},
		node2:    {ID: node2, LastNet: "subnet-A"},
		destNode: {ID: destNode, LastNet: "subnet-B"},
	}

	// Placement with clumping invariant: max 1 piece per subnet.
	fixer.placements = nodeselection.PlacementDefinitions{
		0: {
			ID:        0,
			Invariant: nodeselection.ClumpingByAttribute(nodeselection.LastNetAttribute, 1),
		},
	}

	// Mock selector returns destNode.
	fixer.selectors = map[storj.PlacementConstraint]nodeselection.NodeSelector{
		0: func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
			return []*nodeselection.SelectedNode{
				{ID: destNode, LastNet: "subnet-B"},
			}, nil
		},
	}

	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: node1},
			{Number: 1, StorageNode: node2},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &invariantFork{observer: fixer}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	require.Len(t, fork.jobs, 1)

	// Flush jobs to Redis for verification.
	err = client.PushBatch(ctx, "invariant", fork.jobs)
	require.NoError(t, err)

	var job Job
	ok, err := client.Pop(ctx, "invariant", &job, time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, segment.StreamID, job.StreamID)
	// The second piece (node2) is the violating one (clumping marks the second occurrence).
	assert.Equal(t, node2, job.SourceNode)
	assert.Equal(t, destNode, job.DestNode)
}

func TestInvariantCleanSegment(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fixer, _, cleanup := newTestInvariantObserver(t, ctx)
	defer cleanup()

	node1 := testrand.NodeID()
	node2 := testrand.NodeID()

	fixer.nodeMap = map[storj.NodeID]*nodeselection.SelectedNode{
		node1: {ID: node1, LastNet: "subnet-A"},
		node2: {ID: node2, LastNet: "subnet-B"},
	}

	fixer.placements = nodeselection.PlacementDefinitions{
		0: {
			ID:        0,
			Invariant: nodeselection.ClumpingByAttribute(nodeselection.LastNetAttribute, 1),
		},
	}

	fixer.selectors = map[storj.PlacementConstraint]nodeselection.NodeSelector{
		0: func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
			t.Fatal("selector should not be called for clean segment")
			return nil, nil
		},
	}

	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: node1},
			{Number: 1, StorageNode: node2},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &invariantFork{observer: fixer}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	assert.Empty(t, fork.jobs)
}

func TestInvariantUnknownPlacement(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fixer, _, cleanup := newTestInvariantObserver(t, ctx)
	defer cleanup()

	node1 := testrand.NodeID()

	fixer.nodeMap = map[storj.NodeID]*nodeselection.SelectedNode{
		node1: {ID: node1, LastNet: "subnet-A"},
	}

	fixer.placements = nodeselection.PlacementDefinitions{}
	fixer.selectors = map[storj.PlacementConstraint]nodeselection.NodeSelector{}

	segment := rangedloop.Segment{
		StreamID:  testrand.UUID(),
		Placement: 99,
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: node1},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 1, TotalShares: 1, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &invariantFork{observer: fixer}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	assert.Empty(t, fork.jobs)
}

func TestInvariantNilInvariant(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fixer, _, cleanup := newTestInvariantObserver(t, ctx)
	defer cleanup()

	node1 := testrand.NodeID()

	fixer.nodeMap = map[storj.NodeID]*nodeselection.SelectedNode{
		node1: {ID: node1, LastNet: "subnet-A"},
	}

	fixer.placements = nodeselection.PlacementDefinitions{
		0: {
			ID:        0,
			Invariant: nil,
		},
	}
	fixer.selectors = map[storj.PlacementConstraint]nodeselection.NodeSelector{
		0: func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
			t.Fatal("selector should not be called when invariant is nil")
			return nil, nil
		},
	}

	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: node1},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 1, TotalShares: 1, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &invariantFork{observer: fixer}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	assert.Empty(t, fork.jobs)
}

func TestInvariantReplacementFailsInvariant(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fixer, _, cleanup := newTestInvariantObserver(t, ctx)
	defer cleanup()

	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	badReplacement := testrand.NodeID()

	fixer.nodeMap = map[storj.NodeID]*nodeselection.SelectedNode{
		node1:          {ID: node1, LastNet: "subnet-A"},
		node2:          {ID: node2, LastNet: "subnet-A"},
		badReplacement: {ID: badReplacement, LastNet: "subnet-A"},
	}

	fixer.placements = nodeselection.PlacementDefinitions{
		0: {
			ID:        0,
			Invariant: nodeselection.ClumpingByAttribute(nodeselection.LastNetAttribute, 1),
		},
	}

	// Selector returns a node on the same subnet — won't reduce violations.
	fixer.selectors = map[storj.PlacementConstraint]nodeselection.NodeSelector{
		0: func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
			return []*nodeselection.SelectedNode{
				{ID: badReplacement, LastNet: "subnet-A"},
			}, nil
		},
	}

	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: node1},
			{Number: 1, StorageNode: node2},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &invariantFork{observer: fixer}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	// No jobs since the replacement doesn't reduce violations.
	assert.Empty(t, fork.jobs)
}

func TestInvariantMultipleViolationsTryNext(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fixer, _, cleanup := newTestInvariantObserver(t, ctx)
	defer cleanup()

	// Three nodes on the same subnet: clumping(max 1) marks pieces 1 and 2.
	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	node3 := testrand.NodeID()
	destNode := testrand.NodeID()

	fixer.nodeMap = map[storj.NodeID]*nodeselection.SelectedNode{
		node1:    {ID: node1, LastNet: "subnet-A"},
		node2:    {ID: node2, LastNet: "subnet-A"},
		node3:    {ID: node3, LastNet: "subnet-A"},
		destNode: {ID: destNode, LastNet: "subnet-B"},
	}

	fixer.placements = nodeselection.PlacementDefinitions{
		0: {
			ID:        0,
			Invariant: nodeselection.ClumpingByAttribute(nodeselection.LastNetAttribute, 1),
		},
	}

	callCount := 0
	fixer.selectors = map[storj.PlacementConstraint]nodeselection.NodeSelector{
		0: func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
			callCount++
			if callCount == 1 {
				// First call (for piece 1): return no replacement.
				return nil, nil
			}
			// Second call (for piece 2): return a good replacement.
			return []*nodeselection.SelectedNode{
				{ID: destNode, LastNet: "subnet-B"},
			}, nil
		},
	}

	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: node1},
			{Number: 1, StorageNode: node2},
			{Number: 2, StorageNode: node3},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 2, OptimalShares: 3, TotalShares: 3, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &invariantFork{observer: fixer}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	// First violating piece (piece 1) couldn't be replaced (nil return).
	// Second violating piece (piece 2) got a good replacement.
	require.Len(t, fork.jobs, 1)
	assert.Equal(t, 2, callCount)

	job := fork.jobs[0].(Job)
	assert.Equal(t, node3, job.SourceNode)
	assert.Equal(t, destNode, job.DestNode)
}

func TestInvariantOnlyOnePiecePerSegment(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fixer, _, cleanup := newTestInvariantObserver(t, ctx)
	defer cleanup()

	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	node3 := testrand.NodeID()
	destNode := testrand.NodeID()

	fixer.nodeMap = map[storj.NodeID]*nodeselection.SelectedNode{
		node1:    {ID: node1, LastNet: "subnet-A"},
		node2:    {ID: node2, LastNet: "subnet-A"},
		node3:    {ID: node3, LastNet: "subnet-A"},
		destNode: {ID: destNode, LastNet: "subnet-B"},
	}

	fixer.placements = nodeselection.PlacementDefinitions{
		0: {
			ID:        0,
			Invariant: nodeselection.ClumpingByAttribute(nodeselection.LastNetAttribute, 1),
		},
	}

	callCount := 0
	fixer.selectors = map[storj.PlacementConstraint]nodeselection.NodeSelector{
		0: func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
			callCount++
			return []*nodeselection.SelectedNode{
				{ID: destNode, LastNet: "subnet-B"},
			}, nil
		},
	}

	// Three pieces on the same subnet: pieces 1 and 2 violate.
	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: node1},
			{Number: 1, StorageNode: node2},
			{Number: 2, StorageNode: node3},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 2, OptimalShares: 3, TotalShares: 3, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &invariantFork{observer: fixer}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	// Only one job even though two pieces violate.
	require.Len(t, fork.jobs, 1)
	assert.Equal(t, 1, callCount)
}

func TestInvariantBatchFlush(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fixer, client, cleanup := newTestInvariantObserver(t, ctx)
	defer cleanup()

	drainNodeSubnet := "subnet-A"
	normalNode := testrand.NodeID()
	fixer.nodeMap = map[storj.NodeID]*nodeselection.SelectedNode{
		normalNode: {ID: normalNode, LastNet: "subnet-B"},
	}

	// For each segment, we need two nodes on the same subnet.
	// We'll create unique node IDs per segment but they all map to subnet-A.

	fixer.placements = nodeselection.PlacementDefinitions{
		0: {
			ID:        0,
			Invariant: nodeselection.ClumpingByAttribute(nodeselection.LastNetAttribute, 1),
		},
	}

	fixer.selectors = map[storj.PlacementConstraint]nodeselection.NodeSelector{
		0: func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
			return []*nodeselection.SelectedNode{
				{ID: testrand.NodeID(), LastNet: "subnet-C"},
			}, nil
		},
	}

	var segments []rangedloop.Segment
	for i := 0; i < 12; i++ {
		n1 := testrand.NodeID()
		n2 := testrand.NodeID()
		fixer.nodeMap[n1] = &nodeselection.SelectedNode{ID: n1, LastNet: drainNodeSubnet}
		fixer.nodeMap[n2] = &nodeselection.SelectedNode{ID: n2, LastNet: drainNodeSubnet}

		segments = append(segments, rangedloop.Segment{
			StreamID:    testrand.UUID(),
			RootPieceID: testrand.PieceID(),
			Pieces: metabase.Pieces{
				{Number: 0, StorageNode: n1},
				{Number: 1, StorageNode: n2},
			},
			Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
			EncryptedSize: 1024,
		})
	}

	// Pre-initialize the consumer group.
	err := client.Push(ctx, "invariant", Job{})
	require.NoError(t, err)
	var warmup Job
	ok, err := client.Pop(ctx, "invariant", &warmup, time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	fork := &invariantFork{observer: fixer}
	err = fork.Process(ctx, segments)
	require.NoError(t, err)

	assert.Empty(t, fork.jobs, "all jobs should have been flushed")

	// Verify all 12 jobs are in the queue.
	for i := 0; i < 12; i++ {
		var job Job
		ok, err := client.Pop(ctx, "invariant", &job, time.Second)
		require.NoError(t, err)
		require.True(t, ok, "expected job %d to be in queue", i)
	}

	// No more jobs.
	var extra Job
	ok, err = client.Pop(ctx, "invariant", &extra, 100*time.Millisecond)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestInvariantPlacementFilter(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fixer, _, cleanup := newTestInvariantObserver(t, ctx)
	defer cleanup()

	fixer.config.Placement = 5

	node1 := testrand.NodeID()
	node2 := testrand.NodeID()

	fixer.nodeMap = map[storj.NodeID]*nodeselection.SelectedNode{
		node1: {ID: node1, LastNet: "subnet-A"},
		node2: {ID: node2, LastNet: "subnet-A"},
	}

	fixer.placements = nodeselection.PlacementDefinitions{
		0: {
			ID:        0,
			Invariant: nodeselection.ClumpingByAttribute(nodeselection.LastNetAttribute, 1),
		},
	}
	fixer.selectors = map[storj.PlacementConstraint]nodeselection.NodeSelector{
		0: func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
			t.Fatal("selector should not be called for wrong placement")
			return nil, nil
		},
	}

	// Segment has placement 0, but config filters for placement 5.
	segment := rangedloop.Segment{
		StreamID:  testrand.UUID(),
		Placement: 0,
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: node1},
			{Number: 1, StorageNode: node2},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &invariantFork{observer: fixer}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	assert.Empty(t, fork.jobs)
}

func TestInvariantFilterInvariant(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fixer, _, cleanup := newTestInvariantObserver(t, ctx)
	defer cleanup()

	// Use FilterInvariant: only nodes with country "US" are valid.
	usNode := testrand.NodeID()
	deNode := testrand.NodeID()
	destNode := testrand.NodeID()

	fixer.nodeMap = map[storj.NodeID]*nodeselection.SelectedNode{
		usNode:   {ID: usNode, CountryCode: location.UnitedStates},
		deNode:   {ID: deNode, CountryCode: location.Germany},
		destNode: {ID: destNode, CountryCode: location.UnitedStates},
	}

	usFilter := nodeselection.NewCountryFilter(location.NewSet(location.UnitedStates))

	fixer.placements = nodeselection.PlacementDefinitions{
		0: {
			ID:        0,
			Invariant: nodeselection.FilterInvariant(usFilter),
		},
	}

	fixer.selectors = map[storj.PlacementConstraint]nodeselection.NodeSelector{
		0: func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
			return []*nodeselection.SelectedNode{
				{ID: destNode, CountryCode: location.UnitedStates},
			}, nil
		},
	}

	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: usNode},
			{Number: 1, StorageNode: deNode},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &invariantFork{observer: fixer}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	require.Len(t, fork.jobs, 1)
	job := fork.jobs[0].(Job)
	assert.Equal(t, deNode, job.SourceNode)
	assert.Equal(t, destNode, job.DestNode)
}
