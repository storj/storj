// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeaudit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
)

func TestColdLegacyStatProcess(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Create nodes.
	nodeValid := testrand.NodeID()   // valid for placement 0
	nodeInvalid := testrand.NodeID() // tracked but NOT valid for placement 0
	nodeUntracked := testrand.NodeID()

	// Set up observer with pre-populated state (bypassing Start which needs overlay).
	observer := &ColdLegacyStat{
		log:       zaptest.NewLogger(t),
		config:    ColdLegacyStatConfig{},
		nodeStats: make(map[storj.NodeID]*NodeStats),
		nodeCache: map[storj.NodeID]nodeselection.SelectedNode{
			nodeValid:     {ID: nodeValid},
			nodeInvalid:   {ID: nodeInvalid},
			nodeUntracked: {ID: nodeUntracked},
		},
		validPlacements: map[storj.NodeID]map[storj.PlacementConstraint]bool{
			nodeValid:   {0: true, 1: true},
			nodeInvalid: {1: true}, // valid for placement 1 only, NOT placement 0
			// nodeUntracked has no entry — not tracked by the filter
		},
	}

	// Segment on placement 0: PieceSize() with stripe alignment gives pieceSize=20.
	segments := []rangedloop.Segment{
		{
			StreamID:      testrand.UUID(),
			RootPieceID:   testrand.PieceID(),
			EncryptedSize: 290,
			Placement:     0,
			Redundancy: storj.RedundancyScheme{
				RequiredShares: 29,
				ShareSize:      10,
			},
			Pieces: metabase.Pieces{
				{Number: 0, StorageNode: nodeValid},
				{Number: 1, StorageNode: nodeInvalid},
				{Number: 2, StorageNode: nodeUntracked},
			},
		},
	}

	// Fork, process, join.
	partial, err := observer.Fork(ctx)
	require.NoError(t, err)

	err = partial.Process(ctx, segments)
	require.NoError(t, err)

	err = observer.Join(ctx, partial)
	require.NoError(t, err)

	err = observer.Finish(ctx)
	require.NoError(t, err)

	// nodeValid is valid for placement 0 — no invalid pieces.
	assert.Nil(t, observer.nodeStats[nodeValid])

	// nodeInvalid is tracked but NOT valid for placement 0 — 1 invalid piece of size 20.
	require.NotNil(t, observer.nodeStats[nodeInvalid])
	assert.Equal(t, int64(1), observer.nodeStats[nodeInvalid].InvalidPieceCount)
	assert.Equal(t, int64(20), observer.nodeStats[nodeInvalid].InvalidPieceBytes)

	// nodeUntracked has no validPlacements entry — should be skipped entirely.
	assert.Nil(t, observer.nodeStats[nodeUntracked])
}
