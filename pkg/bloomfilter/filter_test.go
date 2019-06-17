// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"os"
	"testing"

	"storj.io/storj/pkg/storj"
)

var pieceIDs []storj.PieceID
var nbPiecesInFilter int
var totalNbPieces int
var falsePositiveProbability float64

//  generates 1 million piece ids
// adds 95% of them to the bloom filter,
// and then checks all 1 million piece ids with the bloom filter

func TestMain(m *testing.M) {
	totalNbPieces = 1000000
	nbPiecesInFilter = 950000
	pieceIDs = GenerateIDs(totalNbPieces)
	falsePositiveProbability = 0.1
	os.Exit(m.Run())
}

func TestNoFalsePositive(t *testing.T) {
	filter := NewFilter(len(pieceIDs), falsePositiveProbability)
	for _, pieceID := range pieceIDs[:nbPiecesInFilter] {
		filter.Add(pieceID)
	}

	for _, pieceID := range pieceIDs[:nbPiecesInFilter] {
		if !filter.Contains(pieceID) {
			t.Fatal("Filter returns false negative!")
		}
	}
}

// GenerateIDs generates nbPieces piece ids
func GenerateIDs(nbPieces int) []storj.PieceID {
	pieceIDs := make([]storj.PieceID, nbPieces)
	currentNbPieces := 0
	for currentNbPieces < nbPieces {
		newPiece := storj.NewPieceID()
		pieceIDs[currentNbPieces] = newPiece
		currentNbPieces++
	}
	return pieceIDs
}
