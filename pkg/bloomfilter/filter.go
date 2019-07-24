// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"encoding/binary"
	"math"
	"math/rand"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/storj"
)

const (
	version1 = 1
)

// rangeOffsets contains offsets for selecting subranges
// that minimize overlap in the first hash functions
var rangeOffsets = [...]byte{9, 13, 19, 23}

// Filter is a bloom filter implementation
type Filter struct {
	seed      byte
	hashCount byte
	table     []byte
}

// newExplicit returns a new custom filter.
func newExplicit(seed, hashCount byte, sizeInBytes int) *Filter {
	return &Filter{
		seed:      seed,
		hashCount: hashCount,
		table:     make([]byte, sizeInBytes),
	}
}

// NewOptimal returns a filter based on expected element count and false positive rate.
func NewOptimal(expectedElements int, falsePositiveRate float64) *Filter {
	hashCount, sizeInBytes := getHashCountAndSize(expectedElements, falsePositiveRate)
	seed := byte(rand.Intn(255))

	return newExplicit(seed, byte(hashCount), sizeInBytes)
}

// NewOptimalMaxSize returns a filter based on expected element count and false positive rate, capped at a maximum size in bytes
func NewOptimalMaxSize(expectedElements int, falsePositiveRate float64, maxSize memory.Size) *Filter {
	hashCount, sizeInBytes := getHashCountAndSize(expectedElements, falsePositiveRate)
	seed := byte(rand.Intn(255))

	if sizeInBytes > maxSize.Int() {
		sizeInBytes = maxSize.Int()
	}

	return newExplicit(seed, byte(hashCount), sizeInBytes)
}

func getHashCountAndSize(expectedElements int, falsePositiveRate float64) (hashCount, size int) {
	// calculation based on https://en.wikipedia.org/wiki/Bloom_filter#Optimal_number_of_hash_functions
	bitsPerElement := -1.44 * math.Log2(falsePositiveRate)
	hashCount = int(math.Ceil(bitsPerElement * math.Log(2)))
	if hashCount > 32 {
		// it will never be larger, but just in case to avoid overflow
		hashCount = 32
	}
	size = int(math.Ceil(float64(expectedElements) * bitsPerElement / 8))

	return hashCount, size
}

// Parameters returns filter parameters.
func (filter *Filter) Parameters() (hashCount, size int) {
	return int(filter.hashCount), len(filter.table)
}

// Add adds an element to the bloom filter
func (filter *Filter) Add(pieceID storj.PieceID) {
	offset, rangeOffset := initialConditions(filter.seed)

	for k := byte(0); k < filter.hashCount; k++ {
		hash, bit := subrange(offset, pieceID)

		offset += rangeOffset
		if offset >= len(storj.PieceID{}) {
			offset -= len(storj.PieceID{})
		}

		bucket := hash % uint64(len(filter.table))
		filter.table[bucket] |= 1 << (bit % 8)
	}
}

// Contains return true if pieceID may be in the set
func (filter *Filter) Contains(pieceID storj.PieceID) bool {
	offset, rangeOffset := initialConditions(filter.seed)

	for k := byte(0); k < filter.hashCount; k++ {
		hash, bit := subrange(offset, pieceID)

		offset += rangeOffset
		if offset >= len(storj.PieceID{}) {
			offset -= len(storj.PieceID{})
		}

		bucket := hash % uint64(len(filter.table))
		if filter.table[bucket]&(1<<(bit%8)) == 0 {
			return false
		}
	}

	return true
}

func initialConditions(seed byte) (initialOffset, rangeOffset int) {
	initialOffset = int(seed % 32)
	rangeOffset = int(rangeOffsets[int(seed/32)%len(rangeOffsets)])
	return initialOffset, rangeOffset
}

func subrange(seed int, id storj.PieceID) (uint64, byte) {
	if seed > len(id)-9 {
		var unwrap [9]byte
		n := copy(unwrap[:], id[seed:])
		copy(unwrap[n:], id[:])
		return binary.LittleEndian.Uint64(unwrap[:]), unwrap[8]
	}
	return binary.LittleEndian.Uint64(id[seed : seed+8]), id[seed+8]
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

	return filter, nil
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
