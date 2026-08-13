// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package repair_test

import (
	"testing"

	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/repair"
	"storj.io/storj/shared/location"
)

// BenchmarkClassifySegmentPieces measures the per-segment allocation cost of a
// 54-piece segment.
func BenchmarkClassifySegmentPieces(b *testing.B) {
	const pieceCount = 54

	indexes := make([]int, pieceCount)
	for i := range indexes {
		indexes[i] = i
	}

	nodes := generateNodes(pieceCount, func(ix int) bool {
		return true
	}, func(ix int, node *nodeselection.SelectedNode) {
		node.CountryCode = location.UnitedStates
		node.LastNet = "10.0.0.0"
	})
	pieces := createPieces(nodes, indexes...)
	placement := nodeselection.TestPlacementDefinitions()[0]

	b.ReportAllocs()
	for b.Loop() {
		_ = repair.ClassifySegmentPieces(pieces, nodes, nil, true, true, placement)
	}
}
