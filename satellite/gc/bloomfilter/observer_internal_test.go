// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

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

	observer := NewObserver(log, Config{InitialPieces: 123}, nil)
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

	observer.add(testrand.NodeID(), testrand.PieceID())

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
		observer := NewObserver(log, Config{AccessGrant: "test", Bucket: "test", InitialPieces: 10},
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
	observer := NewObserver(log, Config{AccessGrant: "test", Bucket: "test", InitialPieces: 10},
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

type mockOverlay struct {
	pieceCounts map[storj.NodeID]int64
}

func (o *mockOverlay) ActiveNodesPieceCounts(context.Context) (map[storj.NodeID]int64, error) {
	return o.pieceCounts, nil
}
