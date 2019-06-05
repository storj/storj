package filters

import (
	"testing"

	"storj.io/storj/pkg/storj"
)

var pieceIDs [][]byte
var piecesBytes [][]byte
var initDone bool
var nbPiecesInFilter int
var totalNbPieces int
var falsePositiveProbability float64

//  generates 1 million piece ids
// adds 95% of them to the bloom filter,
// and then checks all 1 million piece ids with the bloom filter
// measure times, memory allocation, false positives

func Init() {
	if !initDone {
		totalNbPieces = 1000000
		nbPiecesInFilter = 950000
		pieceIDs = GenerateIDs(totalNbPieces)
		initDone = true
		falsePositiveProbability = 0.01
	}
}

func benchmarkAdd(filter Filter, pieceIDs [][]byte, b *testing.B) {
	for _, pieceID := range pieceIDs {
		filter.Add(pieceID)
	}
}

func benchmarkContains(filter Filter, pieceIDs [][]byte, b *testing.B) (nbPiecesIn int) {
	nbPiecesIn = 0
	for _, pieceID := range pieceIDs {
		if filter.Contains(pieceID) {
			nbPiecesIn++
		}
	}
	return
}

func benchmarkFilter(filter Filter, pieceIDs [][]byte, b *testing.B) {
	b.ReportAllocs()
	benchmarkAdd(filter, pieceIDs[0:nbPiecesInFilter], b)
	nbIn := benchmarkContains(filter, pieceIDs[0:nbPiecesInFilter], b)
	if nbIn < nbPiecesInFilter {
		// we have a false negative - it should not happen
		b.Fail()
	}
	nbIn = benchmarkContains(filter, pieceIDs[nbPiecesInFilter:], b)
	falsePositiveP := float64(nbIn) / float64(len(pieceIDs[nbPiecesInFilter:]))
	if falsePositiveP > falsePositiveProbability {
		b.Log("False positive ratio: ", falsePositiveP, " - greater than expected :", falsePositiveProbability)
	}
}

func BenchmarkFilter1(b *testing.B) {
	b.ReportAllocs()
	Init()
}

/*func BenchmarkFilterNaive(b *testing.B) {
	benchmarkFilter(NewPerfectSet(nbPiecesInFilter), pieceIDs, b)
}*/

func BenchmarkFilterZeebo(b *testing.B) {
	Init()
	benchmarkFilter(NewZeeboBloomFilter(uint(len(pieceIDs)), falsePositiveProbability), pieceIDs, b)
}

func BenchmarkFilterWillf(b *testing.B) {
	Init()
	benchmarkFilter(NewWillfBloomFilter(uint(len(pieceIDs)), falsePositiveProbability), pieceIDs, b)
}

func BenchmarkFilterSteakknife(b *testing.B) {
	Init()
	benchmarkFilter(NewSteakknifeBloomFilter(uint64(len(pieceIDs)), falsePositiveProbability), pieceIDs, b)
}

func BenchmarkFilterCuckoo(b *testing.B) {
	Init()
	benchmarkFilter(NewCuckooFilter(len(pieceIDs)), pieceIDs, b)
}

// GenerateIDs generates nbPieces piece ids
func GenerateIDs(nbPieces int) [][]byte {
	toReturnBytes := make([][]byte, nbPieces)
	currentNbPieces := 0
	for currentNbPieces < nbPieces {
		newPiece := storj.NewPieceID()
		// make sure we don't add the piece id twice
		/*for ArrayContains(newPiece.Bytes(), toReturnBytes) {
			newPiece = storj.NewPieceID()
		}*/
		//toReturnPieces[currentNbPieces] = newPiece
		toReturnBytes[currentNbPieces] = newPiece.Bytes()
		currentNbPieces = currentNbPieces + 1
	}
	return toReturnBytes
}
