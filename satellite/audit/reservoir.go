// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"math/rand"

	"storj.io/storj/pkg/storj"
)

// Reservoir holds a certain number of segments to reflect a random sample
type Reservoir struct {
	Paths   []storj.Path
	Size    int
	NumSeen int
}

// NewReservoir instantiates a Reservoir
func NewReservoir(size int) *Reservoir {
	return &Reservoir{
		Size:    size,
		NumSeen: 0,
	}
}

// sample makes sure that for every segment in metainfo from index i=numSlots..n-1,
// pick a random number r = rand(0..i), and if r < numSlots, replace reservoir.Segments[r] with segment
func (reservoir *Reservoir) sample(path storj.Path) {
	reservoir.NumSeen++
	if len(reservoir.Paths) < reservoir.Size {
		reservoir.Paths = append(reservoir.Paths, path)
	} else {
		random := rand.Intn(reservoir.NumSeen)
		if random < reservoir.Size {
			reservoir.Paths[random] = path
		}
	}
	return
}
