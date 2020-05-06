// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"math/bits"

	"github.com/zeebo/errs"
)

// errorBitmaskInvalidIdx is the error class to return invalid indexes for the
// the bitArray type.
var errorBitmaskInvalidIdx = errs.Class("invalid index")

// bitArray allows easy access to bit values by indices.
type bitArray []byte

// Set tracks index in mask. It returns an error if index is negative.
// Set will resize the array if you access an index larger than its Length.
func (bytes *bitArray) Set(index int) error {
	bitIndex, byteIndex := index%8, index/8
	switch {
	case index < 0:
		return errorBitmaskInvalidIdx.New("negative value (%d)", index)
	case byteIndex >= len(*bytes):
		sizeToGrow := byteIndex - len(*bytes) + 1
		*bytes = append(*bytes, make([]byte, sizeToGrow)...)
	}
	mask := byte(1) << bitIndex
	(*bytes)[byteIndex] |= mask
	return nil
}

// Unset removes bit from index in mask. It returns an error if index is negative.
func (bytes *bitArray) Unset(index int) error {
	bitIndex, byteIndex := index%8, index/8
	switch {
	case index < 0:
		return errorBitmaskInvalidIdx.New("negative value (%d)", index)
	case byteIndex >= len(*bytes):
		return nil
	}
	mask := byte(1) << bitIndex
	(*bytes)[byteIndex] &^= mask
	return nil
}

// Has returns true if the index is tracked in mask otherwise false.
// It returns an error if index is negative.
func (bytes *bitArray) Has(index int) (bool, error) {
	bitIndex, byteIndex := index%8, index/8
	switch {
	case index < 0:
		return false, errorBitmaskInvalidIdx.New("negative value (%d)", index)
	case byteIndex >= len(*bytes):
		return false, nil
	}

	mask := byte(1) << bitIndex
	result := (*bytes)[byteIndex] & mask
	return result != 0, nil
}

// Count returns the number of bits which are set.
func (bytes *bitArray) Count() int {
	count := 0
	for x := 0; x < len(*bytes); x++ {
		count += bits.OnesCount8((*bytes)[x])
	}
	return count
}

// IsSequence returns true if mask has only tracked a correlative sequence of
// indexes starting from index 0.
func (bytes *bitArray) IsSequence() bool {
	ones := bytes.Count()
	zeros := 0
	for byteIndex := len(*bytes) - 1; byteIndex >= 0 && zeros%8 == 0; byteIndex-- {
		zeros = bits.LeadingZeros8((*bytes)[byteIndex])
	}
	return (zeros + ones) == len(*bytes)*8
}

// Length returns the current size of the array in bits.
func (bytes *bitArray) Length() int {
	return len(*bytes) * 8
}
