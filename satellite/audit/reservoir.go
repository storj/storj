// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"math"
	"math/rand"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
)

// Reservoir holds a certain number of segments to reflect a random sample.
type Reservoir struct {
	segments []Segment
	keys     []float64
	size     int8
	index    int8
}

// NewReservoir instantiates a Reservoir.
func NewReservoir(size int) *Reservoir {
	if size < 1 {
		size = 1
	}
	return &Reservoir{
		size:     int8(size),
		index:    0,
		segments: make([]Segment, size),
		keys:     make([]float64, size),
	}
}

// Segments returns the segments picked by the reservoir.
func (reservoir *Reservoir) Segments() []Segment {
	return reservoir.segments[:reservoir.index]
}

// Keys returns the keys for the segments picked by the reservoir.
func (reservoir *Reservoir) Keys() []float64 {
	return reservoir.keys[:reservoir.index]
}

// Sample tries to ensure that each segment passed in has a chance (proportional
// to its size) to be in the reservoir when sampling is complete.
//
// The tricky part is that we do not know ahead of time how many segments will
// be passed in. The way this is accomplished is known as _Reservoir Sampling_.
// The specific algorithm we are using here is called A-Res on the Wikipedia
// article: https://en.wikipedia.org/wiki/Reservoir_sampling#Algorithm_A-Res
func (reservoir *Reservoir) Sample(r *rand.Rand, segment rangedloop.Segment) {
	k := -math.Log(r.Float64()) / float64(segment.EncryptedSize)
	reservoir.sample(k, segment)
}

func (reservoir *Reservoir) sample(k float64, segment rangedloop.Segment) {
	if reservoir.index < reservoir.size {
		reservoir.segments[reservoir.index] = NewSegment(segment)
		reservoir.keys[reservoir.index] = k
		reservoir.index++
	} else {
		max := int8(0)
		for i := int8(1); i < reservoir.size; i++ {
			if reservoir.keys[i] > reservoir.keys[max] {
				max = i
			}
		}
		if k < reservoir.keys[max] {
			reservoir.segments[max] = NewSegment(segment)
			reservoir.keys[max] = k
		}
	}
}

func (reservoir *Reservoir) sampleForMerge(k float64, segment Segment) {
	if reservoir.index < reservoir.size {
		reservoir.segments[reservoir.index] = segment
		reservoir.keys[reservoir.index] = k
		reservoir.index++
	} else {
		max := int8(0)
		for i := int8(1); i < reservoir.size; i++ {
			if reservoir.keys[i] > reservoir.keys[max] {
				max = i
			}
		}
		if k < reservoir.keys[max] {
			reservoir.segments[max] = segment
			reservoir.keys[max] = k
		}
	}
}

// Merge merges the given reservoir into the first. Both reservoirs must have the same size.
func (reservoir *Reservoir) Merge(operand *Reservoir) error {
	if reservoir.size != operand.size {
		return errs.New("cannot merge: mismatched size: expected %d but got %d", reservoir.size, operand.size)
	}
	for i := int8(0); i < operand.index; i++ {
		reservoir.sampleForMerge(operand.keys[i], operand.segments[i])
	}
	return nil
}

// Segment is a segment to audit.
type Segment struct {
	StreamID      uuid.UUID
	Position      metabase.SegmentPosition
	ExpiresAt     *time.Time
	EncryptedSize int32 // size of the whole segment (not a piece)
}

// NewSegment creates a new segment to audit from a metainfo loop segment.
func NewSegment(loopSegment rangedloop.Segment) Segment {
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
