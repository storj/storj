// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"encoding/binary"
	"math"
	"math/big"

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
	f.seed = 8 // TODO allow another seed value
	f.bitsPerElement = int(-1.44*math.Log2(p)) + 1
	f.k = int(float64(f.bitsPerElement)*math.Log(2)) + 1
	m := (expectedElements * f.bitsPerElement) // total number of bits in the array
	f.table = make([]byte, m/8)
	return &f
}

// Add adds an element to the bloom filter
func (cf *Filter) Add(pieceID storj.PieceID) {
	nAsBytes := int2bytes(len(cf.table) * 8)
	n := new(big.Int)
	n.SetBytes(nAsBytes)
	offset := new(big.Int)

	offset = offset.SetBytes(pieceID[cf.seed : cf.seed+cf.k])

	i := 0
	for i < cf.k {
		byteIndex, bitIndex := getByteIndexAndBitIndex(offset, pieceID[cf.seed+i:cf.seed+i+1], n)
		cf.table[byteIndex] |= 0x1 << bitIndex
		i++
	}

}

// Contains return true if pieceID may be in the set
func (cf *Filter) Contains(pieceID storj.PieceID) bool {
	nAsBytes := int2bytes(len(cf.table) * 8)
	n := new(big.Int)
	n.SetBytes(nAsBytes)
	offset := new(big.Int)

	offset = offset.SetBytes(pieceID[cf.seed : cf.seed+cf.k])

	i := 0
	for i < cf.k {
		byteIndex, bitIndex := getByteIndexAndBitIndex(offset, pieceID[cf.seed+i:cf.seed+i+1], n)
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
	b = make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(num))
	return
}

// returns a byte and a bit index within 0 and n-1 computed from an offset and a bit represented as a byte array
func getByteIndexAndBitIndex(offset *big.Int, bit []byte, n *big.Int) (byteIndex uint64, bitIndex uint64) {
	b := new(big.Int)
	b.SetBytes(bit)
	i := offset.Add(offset, b)
	imodn := new(big.Int)
	imodn = imodn.Mod(i, n)

	mod := imodn.Uint64()
	bitIndex = mod % 8
	byteIndex = mod / 8

	return
}
