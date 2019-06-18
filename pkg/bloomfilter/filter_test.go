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
		filter := bloomfilter.NewOptimal(size, 0.1)
		for _, pieceID := range pieceIDs {
			filter.Add(pieceID)
		}
		for _, pieceID := range pieceIDs {
			require.True(t, filter.Contains(pieceID))
		}
	}
}

func TestBytes(t *testing.T) {
	for _, count := range []int{0, 100, 1000, 10000} {
		filter := bloomfilter.NewOptimal(count, 0.1)
		for i := 0; i < count; i++ {
			id := newTestPieceID()
			filter.Add(id)
		}

		bytes := filter.Bytes()
		unmarshaled, err := bloomfilter.NewFromBytes(bytes)
		require.NoError(t, err)

		require.Equal(t, filter, unmarshaled)
	}
}

func TestBytes_Failing(t *testing.T) {
	failing := [][]byte{
		{},
		{0},
		{1},
		{1, 0},
		{255, 10, 10, 10},
	}
	for _, bytes := range failing {
		_, err := bloomfilter.NewFromBytes(bytes)
		require.Error(t, err)
	}
}

// generateTestIDs generates n piece ids
func generateTestIDs(n int) []storj.PieceID {
	ids := make([]storj.PieceID, n)
	for i := range ids {
		ids[i] = newTestPieceID()
	}
	return ids
}

func newTestPieceID() storj.PieceID {
	var id storj.PieceID
	// using math/rand, for less overhead
	_, _ = rand.Read(id[:])
	return id
}

func BenchmarkFilterAdd(b *testing.B) {
	ids := generateTestIDs(100000)
	filter := bloomfilter.NewOptimal(len(ids), 0.1)

	b.Run("Add", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			filter.Add(ids[i%len(ids)])
		}
	})

	b.Run("Contains", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			filter.Contains(ids[i%len(ids)])
		}
	})
}
