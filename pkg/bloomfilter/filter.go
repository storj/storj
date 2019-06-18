// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"encoding/binary"
	"math"
	"math/rand"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
)

const (
	version1 = 1
)

// Filter is a bloom filter implementation
type Filter struct {
	seed      byte
	hashCount byte
	table     []byte
}

// New returns a new custom filter
func New(seed, hashCount byte, sizeInBytes int) *Filter {
	return &Filter{
		seed:      seed,
		hashCount: hashCount,
		table:     make([]byte, sizeInBytes),
	}
}

// NewOptimal returns a filter based on expected element count and false positive rate.
func NewOptimal(expectedElements int, falsePositiveRate float64) *Filter {
	seed := byte(rand.Intn(len(storj.PieceID{})))

	// calculation based on https://en.wikipedia.org/wiki/Bloom_filter#Optimal_number_of_hash_functions
	bitsPerElement := int(-1.44*math.Log2(falsePositiveRate)) + 1
	hashCount := int(float64(bitsPerElement)*math.Log(2)) + 1
	if hashCount > 255 {
		hashCount = 255
	}
	sizeInBytes := expectedElements * bitsPerElement / 8

	return New(seed, byte(hashCount), sizeInBytes)
}

// Add adds an element to the bloom filter
func (filter *Filter) Add(pieceID storj.PieceID) {
	seed := int(filter.seed)
	for k := byte(0); k < filter.hashCount; k++ {
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
	seed := int(filter.seed)
	for k := byte(0); k < filter.hashCount; k++ {
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
