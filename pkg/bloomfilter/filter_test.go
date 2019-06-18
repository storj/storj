// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/storj"
)

func TestNoFalsePositive(t *testing.T) {
	const numberOfPieces = 10000
	pieceIDs := generateTestIDs(numberOfPieces)

	for _, ratio := range []float32{0.5, 1, 2} {
		size := int(numberOfPieces * ratio)
		filter := bloomfilter.New(size, 0.1)
		for _, pieceID := range pieceIDs {
			filter.Add(pieceID)
		}
		for _, pieceID := range pieceIDs {
			require.True(t, filter.Contains(pieceID))
		}
	}
}

// generateTestIDs generates n piece ids
func generateTestIDs(n int) []storj.PieceID {
	ids := make([]storj.PieceID, n)
	for i := range ids {
		// using math/rand, for less overhead
		_, _ = rand.Read(ids[i][:])
	}
	return ids
}
