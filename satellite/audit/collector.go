// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/metaloop"
)

var _ metaloop.Observer = (*Collector)(nil)

// Collector uses the metainfo loop to add segments to node reservoirs.
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
func (collector *Collector) LoopStarted(context.Context, metaloop.LoopInfo) (err error) {
	return nil
}

// RemoteSegment takes a remote segment found in metainfo and creates a reservoir for it if it doesn't exist already.
func (collector *Collector) RemoteSegment(ctx context.Context, segment *metaloop.Segment) (err error) {
	for _, piece := range segment.Pieces {
		if _, ok := collector.Reservoirs[piece.StorageNode]; !ok {
			collector.Reservoirs[piece.StorageNode] = NewReservoir(collector.slotCount)
		}
		collector.Reservoirs[piece.StorageNode].Sample(collector.rand, NewSegment(segment))
	}
	return nil
}

// Object returns nil because the audit service does not interact with objects.
func (collector *Collector) Object(ctx context.Context, object *metaloop.Object) (err error) {
	return nil
}

// InlineSegment returns nil because we're only auditing for storage nodes for now.
func (collector *Collector) InlineSegment(ctx context.Context, segment *metaloop.Segment) (err error) {
	return nil
}
