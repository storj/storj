// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package location

import "math/bits"

const bitsPerBucket = 32

// Set implements a data-structure for fast lookups for country codes.
type Set [(countryCodeCount + bitsPerBucket - 1) / bitsPerBucket]uint32

// NewSet returns a set that has the specific countries set.
func NewSet(countries ...CountryCode) (r Set) {
	for _, c := range countries {
		r.Include(c)
	}
	return r
}

// NewFullSet returns a set that has every bit filled.
func NewFullSet() (r Set) {
	for i := range r {
		r[i] = 0xFFFFFFFF
	}
	return r
}

// Contains checks whether c exists in the set.
func (set *Set) Contains(c CountryCode) bool {
	bucket, bit := c/bitsPerBucket, c%bitsPerBucket
	if int(bucket) >= len(*set) {
		return false
	}
	return set[bucket]&(1<<bit) != 0
}

// Include adds c to the set.
func (set *Set) Include(c CountryCode) {
	bucket, bit := c/bitsPerBucket, c%bitsPerBucket
	if int(bucket) >= len(*set) {
		return
	}
	set[bucket] |= 1 << bit
}

// Remove removes c from the set.
func (set *Set) Remove(c CountryCode) {
	bucket, bit := c/bitsPerBucket, c%bitsPerBucket
	if int(bucket) >= len(*set) {
		return
	}
	set[bucket] &^= 1 << bit
}

// Count returns the number of items in the set.
func (set *Set) Count() int {
	total := 0
	for _, v := range *set {
		total += bits.OnesCount32(v)
	}
	return total
}

// With implements a fluid interface for constructing a set.
func (set Set) With(countries ...CountryCode) Set {
	for _, c := range countries {
		set.Include(c)
	}
	return set
}

// Without implements a fluid interface for constructing a set.
func (set Set) Without(countries ...CountryCode) Set {
	for _, c := range countries {
		set.Remove(c)
	}
	return set
}
