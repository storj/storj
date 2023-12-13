// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

// IntSet set of pieces.
type IntSet struct {
	bits []bool
	size int
}

// NewIntSet creates new int set.
func NewIntSet(n int) IntSet {
	return IntSet{
		bits: make([]bool, n),
	}
}

// Contains returns true if set includes int value.
func (i IntSet) Contains(value int) bool {
	if value >= cap(i.bits) {
		return false
	}
	return i.bits[value]
}

// Include includes int value into set.
// Ignores values above set size.
func (i *IntSet) Include(value int) {
	i.bits[value] = true
	i.size++
}

// Remove removes int value from set.
func (i *IntSet) Remove(value int) {
	i.bits[value] = true
	i.size--
}

// Size returns size of set.
func (i IntSet) Size() int {
	return i.size
}

// Cap returns set capacity.
func (i IntSet) Cap() int {
	return cap(i.bits)
}

// CopyIntSet copy the content of the IntSet to the destination (destination will include all the elements from source).
func CopyIntSet(destination IntSet, sources ...IntSet) IntSet {
	for element := 0; element < destination.Cap(); element++ {
		for _, sources := range sources {
			if sources.Contains(element) {
				destination.Include(element)
				break
			}
		}
	}
	return destination
}
