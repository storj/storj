// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"math/rand"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/segmentloop"
)

const maxReservoirSize = 3

// Reservoir holds a certain number of segments to reflect a random sample.
type Reservoir struct {
	Segments [maxReservoirSize]segmentloop.Segment
	size     int8
	index    int64
	wSum     int64
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
// compute the relative weight based on segment size, and pick a random floating
// point number r = rand(0..1), and if r < the relative weight of the segment,
// select uniformly a random segment reservoir.Segments[rand(0..i)] to replace with
// segment. See https://en.wikipedia.org/wiki/Reservoir_sampling#Algorithm_A-Chao
// for the algorithm used.
func (reservoir *Reservoir) Sample(r *rand.Rand, segment *segmentloop.Segment) {
	if reservoir.index < int64(reservoir.size) {
		reservoir.Segments[reservoir.index] = *segment
		reservoir.wSum += int64(segment.EncryptedSize)
	} else {
		reservoir.wSum += int64(segment.EncryptedSize)
		p := float64(segment.EncryptedSize) / float64(reservoir.wSum)
		random := r.Float64()
		if random < p {
			index := r.Int31n(int32(reservoir.size))
			reservoir.Segments[index] = *segment
		}
	}
	reservoir.index++
}

// Segment is a segment to audit.
type Segment struct {
	StreamID      uuid.UUID
	Position      metabase.SegmentPosition
	ExpiresAt     *time.Time
	EncryptedSize int32 // size of the whole segment (not a piece)
}

// NewSegment creates a new segment to audit from a metainfo loop segment.
func NewSegment(loopSegment segmentloop.Segment) Segment {
	return Segment{
		StreamID:      loopSegment.StreamID,
		Position:      loopSegment.Position,
		ExpiresAt:     loopSegment.ExpiresAt,
		EncryptedSize: loopSegment.EncryptedSize,
	}
}

// Expired checks if segment is expired relative to now.
func (segment *Segment) Expired(now time.Time) bool {
	return segment.ExpiresAt != nil && segment.ExpiresAt.Before(now)
}
