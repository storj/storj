// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"math/rand"
	"storj.io/storj/pkg/pb"
)

// Reservoir holds a certain number of segments to reflect a random sample
type Reservoir struct {
	Segments []*pb.RemoteSegment
	IsVetted bool
	NumSlots int
}

type reservoirConfig struct {
	numVettedSlots   int
	numUnvettedSlots int
}

// NewReservoir instantiates a Reservoir
func NewReservoir(isVetted bool, config reservoirConfig) *Reservoir {
	var numSlots int
	if isVetted {
		numSlots = config.numVettedSlots
	} else {
		numSlots = config.numUnvettedSlots
	}
	return &Reservoir{
		Segments: make([]*pb.RemoteSegment, numSlots),
		NumSlots: numSlots,
	}
}

// sample makes sure that for every segment in metainfo from index i=numSlots..n-1,
// pick a random number r = rand(0..i), and if r < numSlots, replace reservoir.Segments[r] with segment
func (reservoir *Reservoir) sample(segment *pb.RemoteSegment, i int) {
	if len(reservoir.Segments) < reservoir.NumSlots {
		reservoir.Segments = append(reservoir.Segments, segment)
	} else {
		random := rand.Intn(i)
		if random < reservoir.NumSlots {
			reservoir.Segments[random] = segment
		}
	}
	return
}
