// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filters

import (
	"encoding/binary"
	"math"
	"math/big"
)

// CustomFilter is a custom filter
type CustomFilter struct {
	seed              int
	k                 int
	nbBitsPerElements int
	table             []byte
}

// NewCustomFilter returns a new custom filter
func NewCustomFilter(nbElements int, p float64) *CustomFilter {
	var cf CustomFilter
	cf.seed = 8 // TODO allow another seed value
	cf.nbBitsPerElements = int(-1.44*math.Log2(p)) + 1
	cf.k = int(float64(cf.nbBitsPerElements)*math.Log(2)) + 1
	m := (nbElements * cf.nbBitsPerElements) // total number of bits in the array
	cf.table = make([]byte, m/8)
	return &cf
}

// Add adds an element to the bloom filter
func (cf *CustomFilter) Add(pieceID []byte) {
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
func (cf *CustomFilter) Contains(pieceID []byte) bool {
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
func (cf *CustomFilter) Encode() []byte {
	filterInfos := append(int2bytes(cf.seed), int2bytes(cf.nbBitsPerElements)...)
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
