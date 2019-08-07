// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"storj.io/storj/pkg/pb"
)

// Reservoir holds a certain number of segments to reflect a random sample
type Reservoir struct {
	Segments []*pb.RemoteSegment
	IsVetted bool

	Config reservoirConfig
}

type reservoirConfig struct {
	numVettedSlots   int
	numUnvettedSlots int
}

// NewReservoir instantiates a Reservoir
func NewReservoir(isVetted bool, config reservoirConfig) *Reservoir {
	var slots int
	if isVetted {
		slots = config.numVettedSlots
	} else {
		slots = config.numUnvettedSlots
	}
	return &Reservoir{
		Segments: make([]*pb.RemoteSegment, slots),
		Config:   config,
	}
}

func (reservoir *Reservoir) add(segment *pb.RemoteSegment) {
	if reservoir.IsVetted {
		if len(reservoir.Segments) < reservoir.Config.numVettedSlots {
			reservoir.Segments = append(reservoir.Segments, segment)
		}
	} else {
		if len(reservoir.Segments) < reservoir.Config.numUnvettedSlots {
			reservoir.Segments = append(reservoir.Segments, segment)
		}
	}
	// todo: do this step:
	// • For every item in the stream from index i=k..n-1, pick a random number j=rand(0..i), and if j<k, replace reservoir[j] with stream[i]
	return
}
