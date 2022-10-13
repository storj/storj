// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/segmentloop"
)

var _ segmentloop.Observer = (*Collector)(nil)

// Collector uses the segment loop to add segments to node reservoirs.
type Collector struct {
	Reservoirs map[storj.NodeID]*Reservoir
	slotCount  int
	rand       *rand.Rand
}

// NewCollector instantiates a segment collector.
func NewCollector(reservoirSlots int, r *rand.Rand) *Collector {
	return &Collector{
		Reservoirs: make(map[storj.NodeID]*Reservoir),
		slotCount:  reservoirSlots,
		rand:       r,
	}
}

// LoopStarted is called at each start of a loop.
func (collector *Collector) LoopStarted(context.Context, segmentloop.LoopInfo) (err error) {
	return nil
}

// RemoteSegment takes a remote segment found in metainfo and creates a reservoir for it if it doesn't exist already.
func (collector *Collector) RemoteSegment(ctx context.Context, segment *segmentloop.Segment) error {
	// we are expliticy not adding monitoring here as we are tracking loop observers separately

	for _, piece := range segment.Pieces {
		res, ok := collector.Reservoirs[piece.StorageNode]
		if !ok {
			res = NewReservoir(collector.slotCount)
			collector.Reservoirs[piece.StorageNode] = res
		}
		res.Sample(collector.rand, segment)
	}
	return nil
}

// InlineSegment returns nil because we're only auditing for storage nodes for now.
func (collector *Collector) InlineSegment(ctx context.Context, segment *segmentloop.Segment) (err error) {
	return nil
}
