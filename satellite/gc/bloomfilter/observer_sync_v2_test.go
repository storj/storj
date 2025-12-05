// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
)

// TestSyncObserverV2RangeDoesNotPanic ensures that incomplete last
// piece counts do not cause the Range method that Finish uses to panic.
func TestSyncObserverV2RangeDoesNotPanic(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	defer ctx.Check(log.Sync)

	observer := NewSyncObserverV2(log, Config{InitialPieces: 123}, nil)
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
