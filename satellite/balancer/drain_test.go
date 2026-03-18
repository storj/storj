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
)

func newTestDrain(t *testing.T, ctx *testcontext.Context) (*Drain, *taskqueue.Client, func()) {
	redisServer, err := testredis.Start(ctx)
	require.NoError(t, err)

	client, err := taskqueue.NewClient(ctx, taskqueue.Config{
		Address:  "redis://" + redisServer.Addr(),
		Group:    "test-group",
		Consumer: "test-consumer",
	})
	require.NoError(t, err)

	drain := &Drain{
		log:    zaptest.NewLogger(t),
		config: DrainConfig{StreamID: "drain"},
		client: client,
	}

	cleanup := func() {
		require.NoError(t, client.Close())
		require.NoError(t, redisServer.Close())
	}

	return drain, client, cleanup
}

func TestDrainProcessSegment(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	drain, client, cleanup := newTestDrain(t, ctx)
	defer cleanup()

	// Set up nodes: drainNode is being drained, destNode is the replacement.
	drainNode := testrand.NodeID()
	normalNode := testrand.NodeID()
	destNode := testrand.NodeID()

	drain.drainNodes = map[storj.NodeID]bool{
		drainNode: true,
	}

	// Mock selector that always returns destNode.
	drain.selector = func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
		return []*nodeselection.SelectedNode{
			{ID: destNode},
		}, nil
	}

	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: drainNode},
			{Number: 1, StorageNode: normalNode},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &drainFork{observer: drain}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	// Flush remaining jobs.
	require.NotEmpty(t, fork.jobs)
	err = client.PushBatch(ctx, drain.config.StreamID, fork.jobs)
	require.NoError(t, err)

	var job Job
	ok, err := client.Pop(ctx, drain.config.StreamID, &job, time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, segment.StreamID, job.StreamID)
	assert.Equal(t, drainNode, job.SourceNode)
	assert.Equal(t, destNode, job.DestNode)
}

func TestDrainSkipsNonDrainSegment(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	drain, _, cleanup := newTestDrain(t, ctx)
	defer cleanup()

	drainNode := testrand.NodeID()
	normalNode1 := testrand.NodeID()
	normalNode2 := testrand.NodeID()

	drain.drainNodes = map[storj.NodeID]bool{
		drainNode: true,
	}

	// Selector should never be called since no piece is on a drain node.
	drain.selector = func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
		t.Fatal("selector should not be called for non-drain segment")
		return nil, nil
	}

	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: normalNode1},
			{Number: 1, StorageNode: normalNode2},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &drainFork{observer: drain}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	assert.Empty(t, fork.jobs)
}

func TestDrainSkipsWhenSelectorReturnsExistingNode(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	drain, _, cleanup := newTestDrain(t, ctx)
	defer cleanup()

	drainNode := testrand.NodeID()
	normalNode := testrand.NodeID()

	drain.drainNodes = map[storj.NodeID]bool{
		drainNode: true,
	}

	// Selector returns a node already in the segment.
	drain.selector = func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
		return []*nodeselection.SelectedNode{
			{ID: normalNode},
		}, nil
	}

	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: drainNode},
			{Number: 1, StorageNode: normalNode},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &drainFork{observer: drain}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	assert.Empty(t, fork.jobs)
}

func TestDrainSkipsWhenNoReplacementAvailable(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	drain, _, cleanup := newTestDrain(t, ctx)
	defer cleanup()

	drainNode := testrand.NodeID()

	drain.drainNodes = map[storj.NodeID]bool{
		drainNode: true,
	}

	// Selector returns nothing.
	drain.selector = func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
		return nil, nil
	}

	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: drainNode},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 1, TotalShares: 1, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &drainFork{observer: drain}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	assert.Empty(t, fork.jobs)
}

func TestDrainOnlyMovesOnePiecePerSegment(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	drain, _, cleanup := newTestDrain(t, ctx)
	defer cleanup()

	drainNode1 := testrand.NodeID()
	drainNode2 := testrand.NodeID()
	destNode := testrand.NodeID()

	drain.drainNodes = map[storj.NodeID]bool{
		drainNode1: true,
		drainNode2: true,
	}

	callCount := 0
	drain.selector = func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
		callCount++
		return []*nodeselection.SelectedNode{
			{ID: destNode},
		}, nil
	}

	// Segment has two pieces on drain nodes.
	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: drainNode1},
			{Number: 1, StorageNode: drainNode2},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &drainFork{observer: drain}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	// Only one job should be created (one piece per segment per pass).
	require.Len(t, fork.jobs, 1)
	assert.Equal(t, 1, callCount)

	job := fork.jobs[0].(Job)
	assert.Equal(t, drainNode1, job.SourceNode)
	assert.Equal(t, destNode, job.DestNode)
}

func TestDrainPassesExcludedNodes(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	drain, _, cleanup := newTestDrain(t, ctx)
	defer cleanup()

	drainNode := testrand.NodeID()
	normalNode := testrand.NodeID()
	destNode := testrand.NodeID()

	drain.drainNodes = map[storj.NodeID]bool{
		drainNode: true,
	}

	var capturedExcluded []storj.NodeID
	var capturedAlreadySelected []*nodeselection.SelectedNode

	drain.selector = func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
		capturedExcluded = excluded
		capturedAlreadySelected = alreadySelected
		return []*nodeselection.SelectedNode{
			{ID: destNode},
		}, nil
	}

	segment := rangedloop.Segment{
		StreamID: testrand.UUID(),
		Pieces: metabase.Pieces{
			{Number: 0, StorageNode: drainNode},
			{Number: 1, StorageNode: normalNode},
		},
		Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 2, TotalShares: 2, ShareSize: 256},
		EncryptedSize: 1024,
	}

	fork := &drainFork{observer: drain}
	err := fork.processSegment(ctx, &segment)
	require.NoError(t, err)

	// Verify excluded is empty (we use alreadySelected instead).
	require.Empty(t, capturedExcluded)

	// Verify alreadySelected contains the segment nodes.
	require.Len(t, capturedAlreadySelected, 2)
	selectedIDs := make([]storj.NodeID, len(capturedAlreadySelected))
	for i, n := range capturedAlreadySelected {
		selectedIDs[i] = n.ID
	}
	assert.Contains(t, selectedIDs, drainNode)
	assert.Contains(t, selectedIDs, normalNode)
}

func TestDrainBatchFlush(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	drain, client, cleanup := newTestDrain(t, ctx)
	defer cleanup()

	drainNode := testrand.NodeID()

	drain.drainNodes = map[storj.NodeID]bool{
		drainNode: true,
	}

	destCounter := 0
	drain.selector = func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
		destCounter++
		return []*nodeselection.SelectedNode{
			{ID: testrand.NodeID()},
		}, nil
	}

	// Create 12 segments each with a piece on the drain node.
	// This should trigger one batch flush at 10 and leave 2 in the buffer.
	var segments []rangedloop.Segment
	for i := 0; i < 12; i++ {
		segments = append(segments, rangedloop.Segment{
			StreamID:    testrand.UUID(),
			RootPieceID: testrand.PieceID(),
			Pieces: metabase.Pieces{
				{Number: 0, StorageNode: drainNode},
			},
			Redundancy:    storj.RedundancyScheme{RequiredShares: 1, RepairShares: 1, OptimalShares: 1, TotalShares: 1, ShareSize: 256},
			EncryptedSize: 1024,
		})
	}

	// Pre-initialize the consumer group so Pop can read messages pushed during Process.
	err := client.Push(ctx, drain.config.StreamID, Job{})
	require.NoError(t, err)
	var warmup Job
	ok, err := client.Pop(ctx, drain.config.StreamID, &warmup, time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	fork := &drainFork{observer: drain}
	err = fork.Process(ctx, segments)
	require.NoError(t, err)

	// All 12 jobs should have been pushed (10 via batch flush + 2 via final flush in Process).
	assert.Empty(t, fork.jobs, "all jobs should have been flushed")
	assert.Equal(t, 12, destCounter)

	// Verify all 12 jobs are in the queue.
	for i := 0; i < 12; i++ {
		var job Job
		ok, err := client.Pop(ctx, drain.config.StreamID, &job, time.Second)
		require.NoError(t, err)
		require.True(t, ok, "expected job %d to be in queue", i)
	}

	// No more jobs.
	var extra Job
	ok, err = client.Pop(ctx, drain.config.StreamID, &extra, 100*time.Millisecond)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestDrainSkipsInlineSegments(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	drain, _, cleanup := newTestDrain(t, ctx)
	defer cleanup()

	drainNode := testrand.NodeID()

	drain.drainNodes = map[storj.NodeID]bool{
		drainNode: true,
	}

	drain.selector = func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
		t.Fatal("selector should not be called for inline segment")
		return nil, nil
	}

	// Inline segment: no pieces, has inline data.
	segments := []rangedloop.Segment{
		{
			StreamID:      testrand.UUID(),
			Pieces:        nil,
			Redundancy:    storj.RedundancyScheme{},
			EncryptedSize: 256,
		},
	}

	fork := &drainFork{observer: drain}
	err := fork.Process(ctx, segments)
	require.NoError(t, err)

	assert.Empty(t, fork.jobs)
}
