// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"math/bits"

	"github.com/zeebo/errs"
)

// errorBitmaskInvalidIdx is the error class to return invalid indexes for the
// the bitmask type.
var errorBitmaskInvalidIdx = errs.Class("invalid index")

// bitmask allows to track indexes in an optimal way.
type bitmask uint64

// Set tracks index in mask. It returns an error if index is negative or it's
// greater than 63.
func (mask *bitmask) Set(index int) error {
	switch {
	case index < 0:
		return errorBitmaskInvalidIdx.New("negative value (%d)", index)
	case index > 63:
		return errorBitmaskInvalidIdx.New("index is greater than 63 (%d)", index)
	}

	bit := uint64(1) << index
	*mask = bitmask(uint64(*mask) | bit)
	return nil
}

// Has returns true if the index is tracked in mask otherwise false.
// It returns an error if index is negative or it's greater than 63.
func (mask *bitmask) Has(index int) (bool, error) {
	switch {
	case index < 0:
		return false, errorBitmaskInvalidIdx.New("negative value (%d)", index)
	case index > 63:
		return false, errorBitmaskInvalidIdx.New("index is greater than 63 (%d)", index)
	}

	bit := uint64(1) << index
	bit = uint64(*mask) & bit
	return bit != 0, nil
}

// Count returns the number of tracked indexes.
func (mask *bitmask) Count() int {
	return bits.OnesCount64(uint64(*mask))
}

// IsSequence returns true if mask has only tracked a correlative sequence of
// indexes starting from index 0.
func (mask *bitmask) IsSequence() bool {
	ones := mask.Count()
	zeros := bits.LeadingZeros64(uint64(*mask))

	return (zeros + ones) == 64
}
