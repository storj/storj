// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"encoding/binary"
	"math"

	"storj.io/storj/pkg/storj"
)

// Filter is a bloom filter implementation
type Filter struct {
	seed           int
	k              int
	bitsPerElement int
	table          []byte
}

// NewFilter returns a new custom filter
func NewFilter(expectedElements int, p float64) *Filter {
	var f Filter
	f.seed = 8 // TODO allow another seed value - have to handle the reach of the end of the array
	f.bitsPerElement = int(-1.44*math.Log2(p)) + 1
	f.k = int(float64(f.bitsPerElement)*math.Log(2)) + 1
	m := (expectedElements * f.bitsPerElement) // total number of bits in the array
	f.table = make([]byte, m/8)
	return &f
}

// Add adds an element to the bloom filter
func (cf *Filter) Add(pieceID storj.PieceID) {
	offsetAsByteArray := pieceID[cf.seed : cf.seed+cf.k]
	offset := binary.BigEndian.Uint64(append(make([]byte, len(offsetAsByteArray)), offsetAsByteArray...))
	i := 0
	for i < cf.k {
		byteIndex, bitIndex := getByteIndexAndBitIndex(offset, pieceID[cf.seed+i:cf.seed+i+1], uint64(len(cf.table)*8))
		cf.table[byteIndex] |= 0x1 << bitIndex
		i++
	}

}

// Contains return true if pieceID may be in the set
func (cf *Filter) Contains(pieceID storj.PieceID) bool {
	offsetAsByteArray := pieceID[cf.seed : cf.seed+cf.k]
	offset := binary.BigEndian.Uint64(append(make([]byte, len(offsetAsByteArray)), offsetAsByteArray...))

	i := 0
	for i < cf.k {
		byteIndex, bitIndex := getByteIndexAndBitIndex(offset, pieceID[cf.seed+i:cf.seed+i+1], uint64(len(cf.table)*8))
		if (cf.table[byteIndex] & (0x1 << bitIndex)) == 0 {
			return false
		}
		i++
	}

	return true
}

// Encode encodes the filter
func (cf *Filter) Encode() []byte {
	filterInfos := append(int2bytes(cf.seed), int2bytes(cf.bitsPerElement)...)
	filterInfos = append(filterInfos, int2bytes(cf.k)...)
	return append(filterInfos, cf.table...)
}

func int2bytes(num int) (b []byte) {
	b = make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(num))
	return
}

// returns a byte and a bit index within 0 and n-1 computed from an offset and a bit represented as a byte array
func getByteIndexAndBitIndex(offset uint64, bit []byte, n uint64) (byteIndex uint64, bitIndex uint64) {
	b := make([]byte, 8-len(bit))
	bit = append(b, bit...)
	i := offset + binary.BigEndian.Uint64(bit)
	mod := i % n
	bitIndex = mod % 8
	byteIndex = mod / 8

	return
}
