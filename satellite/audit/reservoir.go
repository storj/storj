// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"math/rand"

	"storj.io/storj/pkg/storj"
)

// Reservoir holds a certain number of segments to reflect a random sample
type Reservoir struct {
	Paths []storj.Path
	Size  int
	index int64
}

// NewReservoir instantiates a Reservoir
func NewReservoir(size int) *Reservoir {
	return &Reservoir{
		Size:  size,
		index: 0,
	}
}

// Sample makes sure that for every segment in metainfo from index i=size..n-1,
// pick a random number r = rand(0..i), and if r < size, replace reservoir.Segments[r] with segment
func (reservoir *Reservoir) Sample(path storj.Path) {
	reservoir.index++
	if len(reservoir.Paths) < reservoir.Size {
		reservoir.Paths = append(reservoir.Paths, path)
	} else {
		random := rand.Intn(int(reservoir.index))
		if random < reservoir.Size {
			reservoir.Paths[random] = path
		}
	}
}
