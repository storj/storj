// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
package filters

import (
	"fmt"
	"os"
	"testing"

	"storj.io/storj/pkg/storj"
)

var pieceIDs [][]byte
var initDone bool
var nbPiecesInFilter int
var totalNbPieces int
var falsePositiveProbability float64

//  generates 1 million piece ids
// adds 95% of them to the bloom filter,
// and then checks all 1 million piece ids with the bloom filter
// measure times, memory allocation, false positives

func TestMain(m *testing.M) {
	totalNbPieces = 1000000
	nbPiecesInFilter = 950000
	pieceIDs = GenerateIDs(totalNbPieces)
	initDone = true
	falsePositiveProbability = 0.1
	m.Run()
}

func BenchmarkCustomFilter(b *testing.B) {
	b.ReportAllocs()
	benchmarkFilter(b, NewCustomFilter(len(pieceIDs), falsePositiveProbability), pieceIDs)
}

func BenchmarkFilterZeebo(b *testing.B) {
	b.ReportAllocs()
	benchmarkFilter(b, NewZeeboBloomFilter(uint(len(pieceIDs)), falsePositiveProbability), pieceIDs)
}

func BenchmarkFilterWillf(b *testing.B) {
	b.ReportAllocs()
	benchmarkFilter(b, NewWillfBloomFilter(uint(len(pieceIDs)), falsePositiveProbability), pieceIDs)
}

func BenchmarkFilterSteakknife(b *testing.B) {
	b.ReportAllocs()
	benchmarkFilter(b, NewSteakknifeBloomFilter(uint64(len(pieceIDs)), falsePositiveProbability), pieceIDs)
}

func BenchmarkFilterCuckoo(b *testing.B) {
	b.ReportAllocs()
	benchmarkFilter(b, NewCuckooFilter(len(pieceIDs)), pieceIDs)
}

func BenchmarkEncodedSize(b *testing.B) {
	file, err := os.Create("test.txt")
	if err != nil {
		fmt.Println(err)
		b.Fail()
	}
	defer func() {
		err := file.Close()
		if err != nil {
			b.Fatal(err.Error())
		}
	}()

	names := []string{"Zeebo", "Willf", "Steakknife", "Custom"}

	_, err = file.WriteString("# p\t")
	if err != nil {
		b.Fatal(err.Error())
	}
	for _, name := range names {
		_, err = file.WriteString(fmt.Sprintf("%s\t\t\t", name))
		if err != nil {
			b.Fatal(err.Error())
		}
	}
	_, err = file.WriteString("\n")
	if err != nil {
		b.Fatal(err.Error())
	}

	for range names {
		_, err = file.WriteString("\t\tsize\treal_p\t")
		if err != nil {
			b.Fatal(err.Error())
		}
	}
	_, err = file.WriteString("\n")
	if err != nil {
		b.Fatal(err.Error())
	}

	p := 0.01
	for p <= 0.21 {
		_, err = file.WriteString(fmt.Sprintf("%.2f\t", p))
		if err != nil {
			b.Fatal(err.Error())
		}
		filters := make([]Filter, 4)
		filters[0] = NewZeeboBloomFilter(uint(len(pieceIDs)), p)
		filters[1] = NewWillfBloomFilter(uint(len(pieceIDs)), p)
		filters[2] = NewSteakknifeBloomFilter(uint64(len(pieceIDs)), p)
		filters[3] = NewCustomFilter(len(pieceIDs), p)

		for _, f := range filters {
			realP := benchmarkFilter(b, f, pieceIDs)
			size := benchmarkEncode(b, f, pieceIDs)
			_, err = file.WriteString(fmt.Sprintf("%d\t%.2f\t", size, realP))
			if err != nil {
				b.Fatal(err.Error())
			}
		}
		_, err = file.WriteString("\n")
		if err != nil {
			b.Fatal(err.Error())
		}
		p += 0.01
	}
}

func benchmarkAdd(b *testing.B, filter Filter, pieceIDs [][]byte) {
	b.Helper()
	for _, pieceID := range pieceIDs {
		filter.Add(pieceID)
	}
}

func benchmarkContains(b *testing.B, filter Filter, pieceIDs [][]byte) (nbPiecesIn int) {
	b.Helper()
	nbPiecesIn = 0
	for _, pieceID := range pieceIDs {
		if filter.Contains(pieceID) {
			nbPiecesIn++
		}
	}
	return
}

func benchmarkFilter(b *testing.B, filter Filter, pieceIDs [][]byte) (p float64) {
	b.Helper()
	benchmarkAdd(b, filter, pieceIDs[0:nbPiecesInFilter])
	nbIn := benchmarkContains(b, filter, pieceIDs[0:nbPiecesInFilter])
	if nbIn < nbPiecesInFilter {
		// we have a false negative - it should not happen
		b.Fatal("False negative!")
	}
	nbIn = benchmarkContains(b, filter, pieceIDs[nbPiecesInFilter:])
	falsePositiveP := float64(nbIn) / float64(len(pieceIDs[nbPiecesInFilter:]))
	if falsePositiveP > falsePositiveProbability {
		b.Log("False positive ratio: ", falsePositiveP, " - greater than expected :", falsePositiveProbability)
	} else {
		b.Log("False positive ratio: ", falsePositiveP)
	}
	return falsePositiveP
}

func benchmarkEncode(b *testing.B, filter Filter, pieceIDs [][]byte) int {
	b.Helper()
	benchmarkAdd(b, filter, pieceIDs[0:nbPiecesInFilter])
	filterAsBytes := filter.Encode()
	return len(filterAsBytes)
}

// GenerateIDs generates nbPieces piece ids
func GenerateIDs(nbPieces int) [][]byte {
	toReturnBytes := make([][]byte, nbPieces)
	currentNbPieces := 0
	for currentNbPieces < nbPieces {
		newPiece := storj.NewPieceID()
		toReturnBytes[currentNbPieces] = newPiece.Bytes()
		currentNbPieces++
	}
	return toReturnBytes
}
