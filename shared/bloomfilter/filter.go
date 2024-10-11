// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package bloomfilter implements a bloom-filter for pieces that need to be preserved.
package bloomfilter

import (
	"encoding/binary"
	"math"
	"math/bits"
	"math/rand"

	"github.com/bmkessler/fastdiv"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/storj"
)

const (
	version1 = 1
)

// rangeOffsets contains offsets for selecting subranges
// that minimize overlap in the first hash functions.
var rangeOffsets = [...]byte{9, 13, 19, 23}

// GenerateSeed returns a pseudo-random seed between 0 and 254.
func GenerateSeed() byte {
	return byte(rand.Intn(255))
}

// Filter is a bloom filter implementation.
type Filter struct {
	seed      byte
	hashCount byte
	table     []byte

	offset      byte
	rangeOffset byte
	tableSize   fastdiv.Uint64
}

// NewExplicit returns a new filter with the explicit seed and parameters.
func NewExplicit(seed, hashCount byte, sizeInBytes int) *Filter {
	offset, rangeOffset := initialConditions(seed)

	return &Filter{
		seed:      seed,
		hashCount: hashCount,
		table:     make([]byte, sizeInBytes),

		offset:      offset,
		rangeOffset: rangeOffset,
		tableSize:   fastdiv.NewUint64(uint64(sizeInBytes)),
	}
}

// NewOptimal returns a filter based on expected element count and false positive rate.
func NewOptimal(expectedElements int64, falsePositiveRate float64) *Filter {
	hashCount, sizeInBytes := OptimalParameters(expectedElements, falsePositiveRate, 0)
	seed := GenerateSeed()

	return NewExplicit(seed, hashCount, sizeInBytes)
}

// NewOptimalMaxSize returns a filter based on expected element count and false positive rate, capped at a maximum size in bytes.
func NewOptimalMaxSize(expectedElements int64, falsePositiveRate float64, maxSize memory.Size) *Filter {
	hashCount, sizeInBytes := OptimalParameters(expectedElements, falsePositiveRate, maxSize)
	seed := GenerateSeed()

	return NewExplicit(seed, hashCount, sizeInBytes)
}

// Parameters returns filter parameters.
func (filter *Filter) Parameters() (hashCount, size int) {
	return int(filter.hashCount), len(filter.table)
}

// SeedAndParameters returns the seed along with the filter parameters.
func (filter *Filter) SeedAndParameters() (seed, hashCount byte, size int) {
	return filter.seed, filter.hashCount, len(filter.table)
}

// Add adds an element to the bloom filter.
func (filter *Filter) Add(pieceID storj.PieceID) {
	var id [len(pieceID) * 2]byte
	copy(id[:], pieceID[:])
	copy(id[len(pieceID):], pieceID[:])

	offset, rangeOffset := filter.offset, filter.rangeOffset
	for h := int(filter.hashCount); h > 0; h-- {
		hash, bit := binary.LittleEndian.Uint64(id[offset:offset+8]), id[offset+8]
		bucket := filter.tableSize.Mod(hash)
		filter.table[bucket] |= 1 << (bit % 8)
		offset = (offset + rangeOffset) % byte(len(storj.PieceID{}))
	}
}

// Contains return true if pieceID may be in the set.
func (filter *Filter) Contains(pieceID storj.PieceID) bool {
	var id [len(pieceID) * 2]byte
	copy(id[:], pieceID[:])
	copy(id[len(pieceID):], pieceID[:])

	offset, rangeOffset := filter.offset, filter.rangeOffset
	for k := byte(0); k < filter.hashCount; k++ {
		hash, bit := binary.LittleEndian.Uint64(id[offset:offset+8]), id[offset+8]
		bucket := filter.tableSize.Mod(hash)
		if filter.table[bucket]&(1<<(bit%8)) == 0 {
			return false
		}
		offset = (offset + rangeOffset) % byte(len(storj.PieceID{}))
	}

	return true
}

func initialConditions(seed byte) (initialOffset, rangeOffset byte) {
	initialOffset = seed % 32
	rangeOffset = rangeOffsets[int(seed/32)%len(rangeOffsets)]
	return initialOffset, rangeOffset
}

// AddFilter adds the given filter into the receiver. The filters
// must have a matching seed and parameters.
func (filter *Filter) AddFilter(operand *Filter) error {
	switch {
	case filter.seed != operand.seed:
		return errs.New("cannot merge: mismatched seed: expected %d but got %d", filter.seed, operand.seed)
	case filter.hashCount != operand.hashCount:
		return errs.New("cannot merge: mismatched hash count: expected %d but got %d", filter.hashCount, operand.hashCount)
	case len(filter.table) != len(operand.table):
		return errs.New("cannot merge: mismatched table size: expected %d but got %d", len(filter.table), len(operand.table))
	}
	for i := 0; i < len(filter.table); i++ {
		filter.table[i] |= operand.table[i]
	}
	return nil
}

// NewFromBytes decodes the filter from a sequence of bytes.
//
// Note: data will be referenced inside the table.
func NewFromBytes(data []byte) (*Filter, error) {
	if len(data) < 3 {
		return nil, errs.New("not enough data")
	}
	if data[0] != version1 {
		return nil, errs.New("unsupported version %d", data[0])
	}

	filter := &Filter{}
	filter.seed = data[1]
	filter.hashCount = data[2]
	filter.table = data[3:]

	if filter.hashCount == 0 {
		return nil, errs.New("invalid hash count %d", filter.hashCount)
	}

	filter.offset, filter.rangeOffset = initialConditions(filter.seed)
	filter.tableSize = fastdiv.NewUint64(uint64(len(filter.table)))

	return filter, nil
}

// FillRate calculates the proportion of bits filled in.
func (filter *Filter) FillRate() float64 {
	filled := uint64(0)
	for _, b := range filter.table {
		filled += uint64(bits.OnesCount8(b))
	}
	return float64(filled) / float64(uint64(len(filter.table))*8)
}

// Bytes encodes the filter into a sequence of bytes that can be transferred on network.
func (filter *Filter) Bytes() []byte {
	bytes := make([]byte, 1+1+1+len(filter.table))
	bytes[0] = version1
	bytes[1] = filter.seed
	bytes[2] = filter.hashCount
	copy(bytes[3:], filter.table)
	return bytes
}

// Size returns the size of Bytes call.
func (filter *Filter) Size() int64 {
	// the first three bytes represent the version, seed, and hash count
	return int64(1 + 1 + 1 + len(filter.table))
}

// OptimalParameters returns the optimal parameters for the given expected
// number of elements, desired false positive rate, and optional maximum
// memory size.
func OptimalParameters(expectedElements int64, falsePositiveRate float64, maxSize memory.Size) (hashCount byte, sizeInBytes int) {
	hashCount, sizeInBytes = getHashCountAndSize(expectedElements, falsePositiveRate)
	if maxSize > 0 && sizeInBytes > maxSize.Int() {
		sizeInBytes = maxSize.Int()
	}
	if hashCount <= 0 {
		hashCount = 1
	}
	if sizeInBytes <= 0 {
		sizeInBytes = 1
	}
	return hashCount, sizeInBytes
}

func getHashCountAndSize(expectedElements int64, falsePositiveRate float64) (hashCount byte, size int) {
	// calculation based on https://en.wikipedia.org/wiki/Bloom_filter#Optimal_number_of_hash_functions
	bitsPerElement := -1.44 * math.Log2(falsePositiveRate)
	hashCountInt := int(math.Ceil(bitsPerElement * math.Log(2)))
	if hashCountInt > 32 {
		// it will never be larger, but just in case to avoid overflow
		hashCountInt = 32
	}
	size = int(math.Ceil(float64(expectedElements) * bitsPerElement / 8))

	return byte(hashCountInt), size
}
