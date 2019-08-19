// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// PathCollector uses the metainfo loop to add paths to node reservoirs
type PathCollector struct {
	Reservoirs     map[storj.NodeID]*Reservoir
	reservoirSlots int
}

// NewPathCollector instantiates a path collector
func NewPathCollector(reservoirSlots int) *PathCollector {
	return &PathCollector{
		Reservoirs:     make(map[storj.NodeID]*Reservoir),
		reservoirSlots: reservoirSlots,
	}
}

// RemoteSegment takes a remote segment found in metainfo and creates a reservoir for it if it doesn't exist already
func (pathCollector *PathCollector) RemoteSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx, path)(&err)

	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		if _, ok := pathCollector.Reservoirs[piece.NodeId]; !ok {
			pathCollector.Reservoirs[piece.NodeId] = NewReservoir(pathCollector.reservoirSlots)
		}
		pathCollector.Reservoirs[piece.NodeId].Sample(path)
	}
	return nil
}

// RemoteObject returns nil because the audit service does not interact with remote objects
func (pathCollector *PathCollector) RemoteObject(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	return nil
}

// InlineSegment returns nil because we're only auditing for storage nodes for now
func (pathCollector *PathCollector) InlineSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	return nil
}
