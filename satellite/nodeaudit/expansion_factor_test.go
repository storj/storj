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
	"storj.io/storj/shared/location"
)

func TestExpansionFactorProcess(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Create 3 online nodes and 1 offline node (not in cache).
	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	node3 := testrand.NodeID()
	offlineNode := testrand.NodeID()

	nodeCache := map[storj.NodeID]nodeselection.SelectedNode{
		node1: {ID: node1, Online: true},
		node2: {ID: node2, Online: true},
		node3: {ID: node3, Online: true},
		// offlineNode intentionally absent from cache
	}

	observer := &ExpansionFactor{
		log:        zaptest.NewLogger(t),
		config:     ExpansionFactorConfig{},
		placements: nodeselection.PlacementDefinitions{},

		placementStats:       make(map[storj.PlacementConstraint]*PlacementExpansionStats),
		nodeCache:            nodeCache,
		excludedCountryCodes: map[location.CountryCode]struct{}{},
	}

	// Segment: encryptedSize=290, shareSize=10, requiredShares=29
	// stripeSize=290, stripes=ceil((290+4)/290)=2, encodedSize=580, pieceSize=580/29=20
	// 4 pieces total, 3 healthy (online), 1 unhealthy (offline/missing).
	segments := []rangedloop.Segment{
		{
			StreamID:      testrand.UUID(),
			RootPieceID:   testrand.PieceID(),
			EncryptedSize: 290,
			Placement:     0,
			Redundancy: storj.RedundancyScheme{
				ShareSize:      10,
				RequiredShares: 29,
				RepairShares:   35,
				OptimalShares:  80,
				TotalShares:    110,
			},
			Pieces: metabase.Pieces{
				{Number: 0, StorageNode: node1},
				{Number: 1, StorageNode: node2},
				{Number: 2, StorageNode: node3},
				{Number: 3, StorageNode: offlineNode},
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

	// Verify stats for placement 0.
	stats := observer.placementStats[0]
	require.NotNil(t, stats)

	assert.Equal(t, int64(1), stats.SegmentCount)
	// TotalSegmentSize = encryptedSize = 290
	assert.Equal(t, int64(290), stats.TotalSegmentSize)
	// pieceSize = 20 (see calculation above); TotalPieceSize = 4 pieces * 20 = 80
	assert.Equal(t, int64(80), stats.TotalPieceSize)
	// HealthySize = 3 healthy pieces * 20 = 60
	assert.Equal(t, int64(60), stats.HealthySize)
}
