// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"math"
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
	Keys     [maxReservoirSize]float64
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

// Sample tries to ensure that each segment passed in has a chance (proportional
// to its size) to be in the reservoir when sampling is complete.
//
// The tricky part is that we do not know ahead of time how many segments will
// be passed in. The way this is accomplished is known as _Reservoir Sampling_.
// The specific algorithm we are using here is called A-Res on the Wikipedia
// article: https://en.wikipedia.org/wiki/Reservoir_sampling#Algorithm_A-Res
func (reservoir *Reservoir) Sample(r *rand.Rand, segment *segmentloop.Segment) {
	k := -math.Log(r.Float64()) / float64(segment.EncryptedSize)
	if reservoir.index < int64(reservoir.size) {
		reservoir.Segments[reservoir.index] = *segment
		reservoir.Keys[reservoir.index] = k
	} else {
		max := 0
		for i := 1; i < int(reservoir.size); i++ {
			if reservoir.Keys[i] > reservoir.Keys[max] {
				max = i
			}
		}
		if k < reservoir.Keys[max] {
			reservoir.Segments[max] = *segment
			reservoir.Keys[max] = k
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
