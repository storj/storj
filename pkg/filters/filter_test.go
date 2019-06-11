package filters

import (
	"fmt"
	"os"
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
		falsePositiveProbability = 0.1
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

func benchmarkFilter(filter Filter, pieceIDs [][]byte, b *testing.B) (p float64) {
	b.ReportAllocs()
	Init()

	benchmarkAdd(filter, pieceIDs[0:nbPiecesInFilter], b)
	nbIn := benchmarkContains(filter, pieceIDs[0:nbPiecesInFilter], b)
	if nbIn < nbPiecesInFilter {
		// we have a false negative - it should not happen
		b.Log("nbIn = ", nbIn)
		b.Fail()
	}
	nbIn = benchmarkContains(filter, pieceIDs[nbPiecesInFilter:], b)
	falsePositiveP := float64(nbIn) / float64(len(pieceIDs[nbPiecesInFilter:]))
	if falsePositiveP > falsePositiveProbability {
		b.Log("False positive ratio: ", falsePositiveP, " - greater than expected :", falsePositiveProbability)
	}
	b.Log("False positive ratio: ", falsePositiveP)
	return falsePositiveP
}

func BenchmarkInit(b *testing.B) {
	Init()
}
func BenchmarkCustomFilter(b *testing.B) {
	Init()
	filter := NewCustomFilter(len(pieceIDs), falsePositiveProbability)
	benchmarkFilter(filter, pieceIDs, b)
}

func BenchmarkFilterZeebo(b *testing.B) {
	Init()
	filter := NewZeeboBloomFilter(uint(len(pieceIDs)), falsePositiveProbability)
	benchmarkFilter(filter, pieceIDs, b)
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

func benchmarkEncode(filter Filter, pieceIDs [][]byte, b *testing.B) int {
	benchmarkAdd(filter, pieceIDs[0:nbPiecesInFilter], b)
	filterAsBytes := filter.Encode()
	return len(filterAsBytes)
}

func BenchmarkEncodedSize(b *testing.B) {
	file, err := os.Create("test.txt")
	if err != nil {
		fmt.Println(err)
		b.Fail()
	}
	defer file.Close()
	Init()

	names := []string{"Zeebo", "Willf", "Steakknife", "Custom"}

	file.WriteString("# p\t")
	for _, name := range names {
		file.WriteString(name)
		file.WriteString("\t")
	}
	file.WriteString("\n")
	p := 0.01
	for p <= 0.21 {
		file.WriteString(fmt.Sprintf("%.2f\t", p))
		filters := make([]Filter, 4)
		filters[0] = NewZeeboBloomFilter(uint(len(pieceIDs)), p)
		filters[1] = NewWillfBloomFilter(uint(len(pieceIDs)), p)
		filters[2] = NewSteakknifeBloomFilter(uint64(len(pieceIDs)), p)
		filters[3] = NewCustomFilter(len(pieceIDs), p)

		for i, f := range filters {
			size := benchmarkEncode(f, pieceIDs, b)
			fmt.Println(names[i], " ", p, " ", size)
			file.WriteString(fmt.Sprintf("%d\t", size))
		}
		file.WriteString("\n")
		p += 0.01
	}
}

// GenerateIDs generates nbPieces piece ids
func GenerateIDs(nbPieces int) [][]byte {
	toReturnBytes := make([][]byte, nbPieces)
	currentNbPieces := 0
	for currentNbPieces < nbPieces {
		newPiece := storj.NewPieceID()
		toReturnBytes[currentNbPieces] = newPiece.Bytes()
		currentNbPieces = currentNbPieces + 1
	}
	return toReturnBytes
}
