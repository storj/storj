// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"encoding/binary"
	"math"
	"math/rand"

	"storj.io/storj/pkg/storj"
)

// Filter is a bloom filter implementation
type Filter struct {
	seed      int
	hashCount int
	table     []byte
}

// New returns a new custom filter
func New(seed, hashCount, sizeInBytes int) *Filter {
	return &Filter{
		seed:      seed,
		hashCount: hashCount,
		table:     make([]byte, sizeInBytes),
	}
}

// NewOptimal returns a filter based on expected element count and false positive rate.
func NewOptimal(expectedElements int, falsePositiveRate float64) *Filter {
	seed := rand.Intn(len(storj.PieceID{}))

	// calculation based on https://en.wikipedia.org/wiki/Bloom_filter#Optimal_number_of_hash_functions
	bitsPerElement := int(-1.44*math.Log2(falsePositiveRate)) + 1
	hashCount := int(float64(bitsPerElement)*math.Log(2)) + 1
	sizeInBytes := expectedElements * bitsPerElement / 8

	return New(seed, hashCount, sizeInBytes)
}

// Add adds an element to the bloom filter
func (filter *Filter) Add(pieceID storj.PieceID) {
	seed := filter.seed
	for k := 0; k < filter.hashCount; k++ {
		hash, bit := subrange(seed, pieceID)
		seed += 11
		if seed > 32 {
			seed -= 32
		}

		offset := hash % uint64(len(filter.table))
		filter.table[offset] |= 1 << (bit % 8)
	}
}

// Contains return true if pieceID may be in the set
func (filter *Filter) Contains(pieceID storj.PieceID) bool {
	seed := filter.seed
	for k := 0; k < filter.hashCount; k++ {
		hash, bit := subrange(seed, pieceID)
		seed += 11
		if seed > 32 {
			seed -= 32
		}

		offset := hash % uint64(len(filter.table))
		if filter.table[offset]&(1<<(bit%8)) == 0 {
			return false
		}
	}

	return true
}

func subrange(offset int, id storj.PieceID) (uint64, byte) {
	if offset > len(id)-9 {
		var unwrap [9]byte
		n := copy(unwrap[:], id[offset:])
		copy(unwrap[n:], id[:])
		return binary.LittleEndian.Uint64(unwrap[:]), unwrap[8]
	}
	return binary.LittleEndian.Uint64(id[offset : offset+8]), id[8]
}

/*
func NewFromBytes() *Filter {}

// Encode encodes the filter
func (filter *Filter) Encode() []byte {
	filterInfos := append(int2bytes(filter.seed), int2bytes(filter.bitsPerElement)...)
	filterInfos = append(filterInfos, int2bytes(filter.hashCount)...)
	return append(filterInfos, filter.table...)
}
*/
