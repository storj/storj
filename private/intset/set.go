// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package intset

import "math/bits"

const (
	bucketBits = 32
)

// Set set of int values.
type Set struct {
	size  int
	bits  []uint32
	count int
}

// NewSet creates new int set.
func NewSet(size int) Set {
	return Set{
		size: size,
		bits: make([]uint32, (size+(bucketBits-1))/bucketBits),
	}
}

// Contains returns true if set includes int value.
// Ignores negative values and above set length.
func (s Set) Contains(value int) bool {
	if value < 0 || value >= s.size {
		return false
	}
	bucket, bit := offset(value)
	return (s.bits[bucket] & (uint32(1) << bit)) != 0
}

// Include includes int value into set.
// Ignores negative values and above set length.
func (s *Set) Include(value int) {
	if value < 0 || value >= s.size {
		return
	}

	if !s.Contains(value) {
		bucket, bit := offset(value)
		s.bits[bucket] |= (uint32(1) << bit)
		s.count++
	}
}

// Exclude removes int value from set.
// Ignores negative values and above set length.
func (s *Set) Exclude(value int) {
	if value < 0 || value >= s.size {
		return
	}
	if s.Contains(value) {
		bucket, bit := offset(value)
		s.bits[bucket] &= ^(uint32(1) << bit)
		s.count--
	}
}

// Count returns number of int values in a set.
func (s Set) Count() int {
	return s.count
}

func offset(value int) (bucket, bit int) {
	return value / bucketBits, value % bucketBits
}

// Add adds to the set content of other sets.
// Sources sets with different size than target will be ignored.
func (s *Set) Add(sources ...Set) {
	s.count = 0
	for k, b := range s.bits {
		for srci := range sources {
			if s.size == sources[srci].size {
				b |= sources[srci].bits[k]
			}
		}
		s.bits[k] = b
		s.count += bits.OnesCount32(s.bits[k])
	}
}
