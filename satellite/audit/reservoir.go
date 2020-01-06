// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"math/rand"

	"storj.io/common/storj"
)

const maxReservoirSize = 3

// Reservoir holds a certain number of segments to reflect a random sample
type Reservoir struct {
	Paths [maxReservoirSize]storj.Path
	size  int8
	index int64
}

// NewReservoir instantiates a Reservoir
func NewReservoir(size int) *Reservoir {
	if size < 1 {
		size = 1
	} else if size > maxReservoirSize {
		size = maxReservoirSize
	}
	return &Reservoir{
		size:  int8(size),
		index: 0,
	}
}

// Sample makes sure that for every segment in metainfo from index i=size..n-1,
// pick a random number r = rand(0..i), and if r < size, replace reservoir.Segments[r] with segment
func (reservoir *Reservoir) Sample(r *rand.Rand, path storj.Path) {
	reservoir.index++
	if reservoir.index < int64(reservoir.size) {
		reservoir.Paths[reservoir.index] = path
	} else {
		random := r.Int63n(reservoir.index)
		if random < int64(reservoir.size) {
			reservoir.Paths[random] = path
		}
	}
}
