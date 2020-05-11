// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"math/bits"

	"github.com/zeebo/errs"
)

// errorBitArrayInvalidIdx is the error class to return invalid indexes for the
// the bitArray type.
var errorBitArrayInvalidIdx = errs.Class("invalid index")

// bitArray allows easy access to bit values by indices.
type bitArray []byte

// Set tracks index in mask. It returns an error if index is negative.
// Set will resize the array if you access an index larger than its Length.
func (bytes *bitArray) Set(index int) error {
	bitIndex, byteIndex := index%8, index/8
	switch {
	case index < 0:
		return errorBitArrayInvalidIdx.New("negative value (%d)", index)
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
		return errorBitArrayInvalidIdx.New("negative value (%d)", index)
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
		return false, errorBitArrayInvalidIdx.New("negative value (%d)", index)
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
	// find the last byte of the sequence that contains some one
	var i int
	for i = len(*bytes) - 1; i >= 0; i-- {
		zeros := bits.LeadingZeros8((*bytes)[i])
		if zeros == 8 {
			continue
		}

		ones := bits.OnesCount8((*bytes)[i])
		if zeros+ones != 8 {
			// zeros and ones in this byte aren't in sequence
			return false
		}

		break
	}

	// The rest of the bytes of the sequence must only contains ones
	i--
	for ; i >= 0; i-- {
		if (*bytes)[i] != 255 {
			return false
		}
	}

	return true
}

// Length returns the current size of the array in bits.
func (bytes *bitArray) Length() int {
	return len(*bytes) * 8
}
