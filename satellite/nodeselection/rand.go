// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import mathrand "math/rand"

// RandomOrder as an iterator of a pseudo-random permutation set.
type RandomOrder struct {
	count uint64
	at    uint64
	prime uint64
	len   uint64
}

// NewRandomOrder creates new iterator, returns number between [0,n) in pseudo-random order.
func NewRandomOrder(n int) RandomOrder {
	if n == 0 {
		return RandomOrder{
			count: 0,
		}
	}
	return RandomOrder{
		count: uint64(n),
		at:    uint64(mathrand.Intn(n)),
		prime: primes[mathrand.Intn(len(primes))],
		len:   uint64(n),
	}
}

// Next generates the next number.
func (r *RandomOrder) Next() bool {
	if r.count == 0 {
		return false
	}
	r.at = (r.at + r.prime) % r.len
	r.count--
	return true
}

// At returns the current number in the permutations.
func (r *RandomOrder) At() uint64 { return r.at }

// Reset makes it possible to reuse the RandomOrder: the full pseudo-random permutations can be read again.
func (r *RandomOrder) Reset() {
	r.count = r.len
}

// Finished returns true, if there is no more permutation.
func (r *RandomOrder) Finished() bool {
	return r.count == 0
}
