// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeaudit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testredis"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/taskqueue"
)

func TestJobSerialization(t *testing.T) {
	ctx := testcontext.New(t)

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
		StreamID:    testrand.UUID(),
		Position:    12345678,
		PieceNo:     42,
		RootPieceID: testrand.PieceID(),
	}

	err = client.Push(ctx, streamID, expected)
	require.NoError(t, err)

	var got Job
	ok, err := client.Pop(ctx, streamID, &got, time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, expected.StreamID, got.StreamID)
	assert.Equal(t, expected.Position, got.Position)
	assert.Equal(t, expected.PieceNo, got.PieceNo)
	assert.Equal(t, expected.RootPieceID, got.RootPieceID)
}

func newTestClient(t *testing.T, ctx *testcontext.Context) (*taskqueue.Client, func()) {
	redisServer, err := testredis.Start(ctx)
	require.NoError(t, err)

	client, err := taskqueue.NewClient(ctx, taskqueue.Config{
		Address:  "redis://" + redisServer.Addr(),
		Group:    "test-group",
		Consumer: "test-consumer",
	})
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, client.Close())
		require.NoError(t, redisServer.Close())
	}
	return client, cleanup
}

func popAllJobs(t *testing.T, ctx *testcontext.Context, client *taskqueue.Client) []Job {
	var jobs []Job
	for {
		var job Job
		ok, err := client.Pop(ctx, streamID, &job, 100*time.Millisecond)
		require.NoError(t, err)
		if !ok {
			break
		}
		jobs = append(jobs, job)
	}
	return jobs
}

func TestPieceListProcess(t *testing.T) {
	ctx := testcontext.New(t)

	targetNodeID := testrand.NodeID()
	otherNodeID := testrand.NodeID()

	client, cleanup := newTestClient(t, ctx)
	defer cleanup()

	pl, err := NewPieceList(client, PieceListConfig{
		Node: targetNodeID.String(),
	})
	require.NoError(t, err)

	require.NoError(t, pl.Start(ctx, time.Now()))

	partial, err := pl.Fork(ctx)
	require.NoError(t, err)

	rootPieceID1 := testrand.PieceID()
	rootPieceID2 := testrand.PieceID()
	streamID1 := testrand.UUID()
	streamID2 := testrand.UUID()
	streamID3 := testrand.UUID()
	pos1 := metabase.SegmentPosition{Part: 0, Index: 0}
	pos2 := metabase.SegmentPosition{Part: 0, Index: 1}

	segments := []rangedloop.Segment{
		{ // remote segment with target node at piece 3
			StreamID:    streamID1,
			Position:    pos1,
			RootPieceID: rootPieceID1,
			Redundancy:  storj.RedundancyScheme{RequiredShares: 29, RepairShares: 35, OptimalShares: 80, TotalShares: 110},
			Pieces: metabase.Pieces{
				{Number: 0, StorageNode: otherNodeID},
				{Number: 3, StorageNode: targetNodeID},
			},
		},
		{ // inline segment (should be skipped)
			StreamID: streamID2,
			Position: pos1,
		},
		{ // remote segment without target node (should be skipped)
			StreamID:    streamID3,
			Position:    pos1,
			RootPieceID: testrand.PieceID(),
			Redundancy:  storj.RedundancyScheme{RequiredShares: 29, RepairShares: 35, OptimalShares: 80, TotalShares: 110},
			Pieces: metabase.Pieces{
				{Number: 0, StorageNode: otherNodeID},
			},
		},
		{ // another remote segment with target node at piece 7
			StreamID:    streamID1,
			Position:    pos2,
			RootPieceID: rootPieceID2,
			Redundancy:  storj.RedundancyScheme{RequiredShares: 29, RepairShares: 35, OptimalShares: 80, TotalShares: 110},
			Pieces: metabase.Pieces{
				{Number: 7, StorageNode: targetNodeID},
				{Number: 1, StorageNode: otherNodeID},
			},
		},
	}

	err = partial.Process(ctx, segments)
	require.NoError(t, err)

	require.NoError(t, pl.Join(ctx, partial))
	require.NoError(t, pl.Finish(ctx))

	jobs := popAllJobs(t, ctx, client)
	require.Len(t, jobs, 2)

	assert.Equal(t, streamID1, jobs[0].StreamID)
	assert.Equal(t, pos1.Encode(), jobs[0].Position)
	assert.Equal(t, uint16(3), jobs[0].PieceNo)
	assert.Equal(t, rootPieceID1, jobs[0].RootPieceID)

	assert.Equal(t, streamID1, jobs[1].StreamID)
	assert.Equal(t, pos2.Encode(), jobs[1].Position)
	assert.Equal(t, uint16(7), jobs[1].PieceNo)
	assert.Equal(t, rootPieceID2, jobs[1].RootPieceID)
}

func TestPieceListProcessBatching(t *testing.T) {
	ctx := testcontext.New(t)

	targetNodeID := testrand.NodeID()

	client, cleanup := newTestClient(t, ctx)
	defer cleanup()

	pl, err := NewPieceList(client, PieceListConfig{
		Node: targetNodeID.String(),
	})
	require.NoError(t, err)

	require.NoError(t, pl.Start(ctx, time.Now()))

	partial, err := pl.Fork(ctx)
	require.NoError(t, err)

	// Create 25 segments that all match the target node.
	// The Process method batches at 10, so this exercises the batching path
	// (two full batches of 10, plus a final batch of 5).
	segments := make([]rangedloop.Segment, 25)
	for i := range segments {
		segments[i] = rangedloop.Segment{
			StreamID:    testrand.UUID(),
			Position:    metabase.SegmentPosition{Part: 0, Index: uint32(i)},
			RootPieceID: testrand.PieceID(),
			Redundancy:  storj.RedundancyScheme{RequiredShares: 29, RepairShares: 35, OptimalShares: 80, TotalShares: 110},
			Pieces: metabase.Pieces{
				{Number: uint16(i), StorageNode: targetNodeID},
			},
		}
	}

	err = partial.Process(ctx, segments)
	require.NoError(t, err)

	require.NoError(t, pl.Join(ctx, partial))
	require.NoError(t, pl.Finish(ctx))

	jobs := popAllJobs(t, ctx, client)
	require.Len(t, jobs, 25)

	for i, job := range jobs {
		assert.Equal(t, segments[i].StreamID, job.StreamID)
		assert.Equal(t, segments[i].Position.Encode(), job.Position)
		assert.Equal(t, uint16(i), job.PieceNo)
		assert.Equal(t, segments[i].RootPieceID, job.RootPieceID)
	}
}
