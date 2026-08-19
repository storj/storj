// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
)

// TestObserverRangeDoesNotPanic ensures that incomplete last
// piece counts do not cause the Range method that Finish uses to panic.
func TestObserverRangeDoesNotPanic(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	defer ctx.Check(log.Sync)

	observer := NewObserver(log, Config{InitialPieces: 123, MaxBloomFilterSize: 2 * memory.MiB}, nil)
	observer.retainInfos = new(concurrentRetainInfos)

	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	node3 := testrand.NodeID()

	observer.lastPieceCounts = map[storj.NodeID]int64{
		node1: 1,
		node2: 2,
		node3: 3,
	}

	observer.add(node1, testrand.PieceID())
	observer.add(node2, testrand.PieceID())
	observer.add(node2, testrand.PieceID())
	observer.add(node3, testrand.PieceID())
	observer.add(node3, testrand.PieceID())
	observer.add(node3, testrand.PieceID())

	// nodes missing from lastPieceCounts are filtered out by Process,
	// so no filter is ever created for them.
	observer.startTime = time.Now()
	require.NoError(t, observer.Process(ctx, []rangedloop.Segment{{
		StreamID:    testrand.UUID(),
		RootPieceID: testrand.PieceID(),
		CreatedAt:   observer.startTime.Add(-time.Hour),
		Pieces:      metabase.Pieces{{Number: 1, StorageNode: testrand.NodeID()}},
	}}))

	var count int
	observer.retainInfos.Range(func(_ storj.NodeID, info *RetainInfo) bool {
		require.NotPanics(t, func() { count += info.Count })
		return true
	})
	require.Equal(t, 6, count)
}

// TestObserverCreationTime verifies that the creation date used for
// the bloom filters is the latest segment creation time seen by any fork,
// and that segments newer than the loop start time are rejected.
func TestObserverCreationTime(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	defer ctx.Check(log.Sync)

	nodeID := testrand.NodeID()
	startTime := time.Now()

	newObserver := func() *Observer {
		observer := NewObserver(log, Config{AccessGrant: "test", Bucket: "test", InitialPieces: 10, MaxBloomFilterSize: 2 * memory.MiB},
			&mockOverlay{pieceCounts: map[storj.NodeID]int64{nodeID: 1}})
		require.NoError(t, observer.Start(ctx, startTime))
		return observer
	}

	segment := func(createdAt time.Time) rangedloop.Segment {
		return rangedloop.Segment{
			StreamID:    testrand.UUID(),
			RootPieceID: testrand.PieceID(),
			CreatedAt:   createdAt,
			Pieces:      metabase.Pieces{{Number: 1, StorageNode: nodeID}},
		}
	}

	t.Run("latest creation time", func(t *testing.T) {
		observer := newObserver()

		require.NoError(t, observer.Process(ctx, []rangedloop.Segment{
			segment(startTime.Add(-3 * time.Hour)),
			segment(startTime.Add(-time.Hour)),
		}))
		require.NoError(t, observer.Process(ctx, []rangedloop.Segment{
			segment(startTime.Add(-2 * time.Hour)),
		}))

		require.Equal(t, startTime.Add(-time.Hour), observer.latestCreationTime)
	})

	t.Run("segment created after loop started", func(t *testing.T) {
		observer := newObserver()

		require.Error(t, observer.Process(ctx, []rangedloop.Segment{
			segment(startTime.Add(time.Hour)),
		}))

		// the ranged loop skips Finish when Process failed, so the observer
		// must still report the failure to the run-once caller.
		require.Error(t, observer.FinishError())
	})
}

// TestObserverStartResetsCounters verifies that the segment counters
// reported when a loop finishes cover only that loop, as the observer is
// reused across ranged loop iterations.
func TestObserverStartResetsCounters(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	defer ctx.Check(log.Sync)

	nodeID := testrand.NodeID()
	observer := NewObserver(log, Config{AccessGrant: "test", Bucket: "test", InitialPieces: 10, MaxBloomFilterSize: 2 * memory.MiB},
		&mockOverlay{pieceCounts: map[storj.NodeID]int64{nodeID: 1}})

	startTime := time.Now()
	require.NoError(t, observer.Start(ctx, startTime))
	require.NoError(t, observer.Process(ctx, []rangedloop.Segment{
		{}, // inline segment, it has no root piece id
		{
			StreamID:    testrand.UUID(),
			RootPieceID: testrand.PieceID(),
			CreatedAt:   startTime.Add(-time.Hour),
			Pieces:      metabase.Pieces{{Number: 1, StorageNode: nodeID}},
		},
	}))
	require.EqualValues(t, 1, observer.inlineCount.Load())
	require.EqualValues(t, 1, observer.remoteCount.Load())

	require.NoError(t, observer.Start(ctx, time.Now()))
	require.Zero(t, observer.inlineCount.Load())
	require.Zero(t, observer.remoteCount.Load())
}

// TestObserverShardRotation verifies that each pass of a run covers one shard
// of the nodes, that the piece counts of the other shards are available again
// on the next pass, and that a rotation is refused outside of a single run.
func TestObserverShardRotation(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	defer ctx.Check(log.Sync)

	pieceCounts := map[storj.NodeID]int64{}
	for range 20 {
		pieceCounts[testrand.NodeID()] = 1
	}
	overlay := &mockOverlay{pieceCounts: pieceCounts}

	config := Config{AccessGrant: "test", Bucket: "test", InitialPieces: 10,
		MaxBloomFilterSize: 2 * memory.MiB, ShardCount: 4, Shard: -1}
	observer := NewObserver(log, config, overlay)

	// a rotation only works when a single run covers every shard
	require.Error(t, observer.Start(ctx, time.Now()))

	seen := map[storj.NodeID]int{}
	passes := 0
	require.NoError(t, observer.RunPasses(ctx, func(ctx context.Context) error {
		if err := observer.Start(ctx, time.Now()); err != nil {
			return err
		}
		require.Equal(t, passes, observer.shard)
		for id := range observer.lastPieceCounts {
			require.NotContains(t, seen, id, "node in more than one shard")
			seen[id] = observer.shard
		}
		passes++
		observer.finishErr = nil // pretend the pass finished
		return nil
	}))
	require.Equal(t, config.ShardCount, passes)
	require.Len(t, seen, len(pieceCounts))
	require.Len(t, overlay.pieceCounts, len(pieceCounts), "the overlay's map was pruned in place")
}

// TestObserverRunPassesStopsOnFailure verifies that a pass which did not
// finish, which the ranged loop only logs, aborts the run instead of leaving
// the generation without that shard.
func TestObserverRunPassesStopsOnFailure(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	defer ctx.Check(log.Sync)

	newObserver := func() *Observer {
		return NewObserver(log, Config{AccessGrant: "test", Bucket: "test", InitialPieces: 10,
			MaxBloomFilterSize: 2 * memory.MiB, ShardCount: 3, Shard: -1},
			&mockOverlay{pieceCounts: map[storj.NodeID]int64{testrand.NodeID(): 1}})
	}

	// a pass that started but never reached Finish, which the ranged loop
	// reports as a successful run
	observer := newObserver()
	passes := 0
	require.Error(t, observer.RunPasses(ctx, func(ctx context.Context) error {
		passes++
		require.NoError(t, observer.Start(ctx, time.Now()))
		return nil
	}))
	require.Equal(t, 1, passes)

	// and a pass that never even reached Start
	observer = newObserver()
	passes = 0
	require.Error(t, observer.RunPasses(ctx, func(ctx context.Context) error {
		passes++
		return nil
	}))
	require.Equal(t, 1, passes)
}

// TestObserverOverlayError verifies that a pass fails when the node list could
// not be loaded, instead of reporting a shard without any filters as done.
func TestObserverOverlayError(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	defer ctx.Check(log.Sync)

	observer := NewObserver(log, Config{AccessGrant: "test", Bucket: "test", InitialPieces: 10,
		MaxBloomFilterSize: 2 * memory.MiB}, &mockOverlay{err: errs.New("overlay is down")})

	require.Error(t, observer.Start(ctx, time.Now()))
	require.Error(t, observer.FinishError())
}

// TestObserverPublishGuard verifies that a failing guard keeps the generation
// from being published and fails the run, rather than being reported after
// LATEST already points at it.
func TestObserverPublishGuard(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	defer ctx.Check(log.Sync)

	nodeID := testrand.NodeID()
	// an unusable access grant, so publishing would be an error rather than a
	// network call
	observer := NewObserver(log, Config{AccessGrant: "test", Bucket: "test", InitialPieces: 10,
		MaxBloomFilterSize: 2 * memory.MiB}, &mockOverlay{pieceCounts: map[storj.NodeID]int64{nodeID: 1}})

	guardErr := errs.New("counts do not add up")
	calls := 0
	observer.SetPublishGuard(func(context.Context) error {
		calls++
		return guardErr
	})

	require.NoError(t, observer.Start(ctx, time.Now()))
	require.NoError(t, observer.Process(ctx, []rangedloop.Segment{{
		StreamID:    testrand.UUID(),
		RootPieceID: testrand.PieceID(),
		CreatedAt:   observer.startTime.Add(-time.Hour),
		Pieces:      metabase.Pieces{{Number: 1, StorageNode: nodeID}},
	}}))
	require.EqualValues(t, 1, observer.ProcessedSegments())

	// the guard error, not an upload error: nothing was uploaded
	require.ErrorIs(t, observer.Finish(ctx), guardErr)
	require.Equal(t, 1, calls)
	// the ranged loop only logs observer errors, so the run-once caller has to
	// see it too
	require.ErrorIs(t, observer.FinishError(), guardErr)
}

// TestCheckConfigSingleShard verifies that rerunning a single shard requires
// the prefix of the generation it belongs to.
func TestCheckConfigSingleShard(t *testing.T) {
	config := Config{AccessGrant: "test", Bucket: "test", ShardCount: 4, Shard: 2}
	require.Error(t, NewUpload(zaptest.NewLogger(t), config).CheckConfig())

	config.UploadPrefix = "2025-01-01T00:00:00Z"
	require.NoError(t, NewUpload(zaptest.NewLogger(t), config).CheckConfig())
}

type mockOverlay struct {
	pieceCounts map[storj.NodeID]int64
	err         error
}

func (o *mockOverlay) ActiveNodesPieceCounts(context.Context) (map[storj.NodeID]int64, error) {
	return o.pieceCounts, o.err
}
