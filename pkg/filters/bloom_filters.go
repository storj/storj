// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filters

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"

	steakknife "github.com/steakknife/bloomfilter"
	willf "github.com/willf/bloom"
	zeebo "github.com/zeebo/sbloom"
)

// ZeeboBloomFilter represents zeebo's bloom filter implementation
type ZeeboBloomFilter struct {
	filter *zeebo.Filter
}

// WillfBloomFilter is willf's bloom filter implementation
type WillfBloomFilter struct {
	filter *willf.BloomFilter
}

// SteakknifeBloomFilter is a bloom filter from steakknife
type SteakknifeBloomFilter struct {
	filter *steakknife.Filter
}

// NewZeeboBloomFilter returns a zeebo bloom filter
func NewZeeboBloomFilter(maxElements uint, p float64) *ZeeboBloomFilter {
	var zbf ZeeboBloomFilter
	zbf.filter = zeebo.NewFilter(fnv.New64(), int(-math.Log(p)/math.Log(2))+1)
	return &zbf
}

// Add adds a pieceID
func (zbf *ZeeboBloomFilter) Add(pieceID []byte) {
	zbf.filter.Add(pieceID)
}

// Contains returns true if the pieceID may be in the set
func (zbf *ZeeboBloomFilter) Contains(pieceID []byte) bool {
	return zbf.filter.Lookup(pieceID)
}

// Encode returns an array of bytes representing the filter
func (zbf *ZeeboBloomFilter) Encode() []byte {
	toReturn, err := zbf.filter.GobEncode()
	if err != nil {
		fmt.Println(err.Error())
		panic("error in gobencode")
	}
	return toReturn
}

// NewWillfBloomFilter returns a bloom filter of size size
func NewWillfBloomFilter(maxElements uint, p float64) *WillfBloomFilter {
	var wbf WillfBloomFilter
	wbf.filter = willf.NewWithEstimates(maxElements, p)
	return &wbf
}

// Add adds pieceID to the set
func (wbf *WillfBloomFilter) Add(pieceID []byte) {
	wbf.filter.Add(pieceID)
}

// Contains return true if pieceID may be in the set
func (wbf *WillfBloomFilter) Contains(pieceID []byte) bool {
	return wbf.filter.Test(pieceID)
}

// Encode returns an array of bytes representing the filter
func (wbf *WillfBloomFilter) Encode() []byte {
	toReturn, _ := wbf.filter.GobEncode()
	return toReturn
}

// NewSteakknifeBloomFilter creates a new SteakknifeBloomFilter
func NewSteakknifeBloomFilter(maxElements uint64, p float64) *SteakknifeBloomFilter {
	var sbf SteakknifeBloomFilter
	sbf.filter, _ = steakknife.NewOptimal(maxElements, p)
	return &sbf
}

type hashableByteArray []byte

func (h hashableByteArray) Write([]byte) (int, error) {
	panic("Unimplemented")
}

func (h hashableByteArray) Sum([]byte) []byte {
	panic("Unimplemented")
}

func (h hashableByteArray) Reset() {
	panic("Unimplemented")
}

func (h hashableByteArray) BlockSize() int {
	panic("Unimplemented")
}

func (h hashableByteArray) Size() int {
	panic("Unimplemented")
}

func (h hashableByteArray) Sum64() uint64 {
	return binary.BigEndian.Uint64(h)
}

// Add adds pieceID to the set
func (sbf *SteakknifeBloomFilter) Add(pieceID []byte) {
	sbf.filter.Add(hashableByteArray(pieceID))
}

// Contains return true if pieceID may be in the set
func (sbf *SteakknifeBloomFilter) Contains(pieceID []byte) bool {
	return sbf.filter.Contains(hashableByteArray(pieceID))
}

// Encode returns an array of bytes representing the filter
func (sbf *SteakknifeBloomFilter) Encode() []byte {
	toReturn, _ := sbf.filter.GobEncode()
	return toReturn
}
