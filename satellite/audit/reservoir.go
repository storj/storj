// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"math/rand"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/metainfo/metaloop"
)

const maxReservoirSize = 3

// Reservoir holds a certain number of segments to reflect a random sample.
type Reservoir struct {
	Segments [maxReservoirSize]Segment
	size     int8
	index    int64
}

// NewReservoir instantiates a Reservoir.
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
// pick a random number r = rand(0..i), and if r < size, replace reservoir.Segments[r] with segment.
func (reservoir *Reservoir) Sample(r *rand.Rand, segment Segment) {
	reservoir.index++
	if reservoir.index < int64(reservoir.size) {
		reservoir.Segments[reservoir.index] = segment
	} else {
		random := r.Int63n(reservoir.index)
		if random < int64(reservoir.size) {
			reservoir.Segments[random] = segment
		}
	}
}

// Segment is a segment to audit.
type Segment struct {
	metabase.SegmentLocation
	StreamID       uuid.UUID
	ExpirationDate time.Time
}

// NewSegment creates a new segment to audit from a metainfo loop segment.
func NewSegment(loopSegment *metaloop.Segment) Segment {
	return Segment{
		SegmentLocation: loopSegment.Location,
		StreamID:        loopSegment.StreamID,
		ExpirationDate:  loopSegment.ExpirationDate,
	}
}

// Expired checks if segment is expired relative to now.
func (segment *Segment) Expired(now time.Time) bool {
	return !segment.ExpirationDate.IsZero() && segment.ExpirationDate.Before(now)
}
