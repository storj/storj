// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metainfo"
)

var _ metainfo.Observer = (*PathCollector)(nil)

// PathCollector uses the metainfo loop to add paths to node reservoirs
//
// architecture: Observer
type PathCollector struct {
	Reservoirs map[storj.NodeID]*Reservoir
	slotCount  int
	rand       *rand.Rand
}

// NewPathCollector instantiates a path collector
func NewPathCollector(reservoirSlots int, r *rand.Rand) *PathCollector {
	return &PathCollector{
		Reservoirs: make(map[storj.NodeID]*Reservoir),
		slotCount:  reservoirSlots,
		rand:       r,
	}
}

// RemoteSegment takes a remote segment found in metainfo and creates a reservoir for it if it doesn't exist already
func (collector *PathCollector) RemoteSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		if _, ok := collector.Reservoirs[piece.NodeId]; !ok {
			collector.Reservoirs[piece.NodeId] = NewReservoir(collector.slotCount)
		}
		collector.Reservoirs[piece.NodeId].Sample(collector.rand, path.Raw)
	}
	return nil
}

// Object returns nil because the audit service does not interact with objects
func (collector *PathCollector) Object(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return nil
}

// InlineSegment returns nil because we're only auditing for storage nodes for now
func (collector *PathCollector) InlineSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return nil
}
