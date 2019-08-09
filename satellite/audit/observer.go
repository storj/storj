// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/overlay"
)

type reservoirConfig struct {
	slotsForVetted   int
	slotsForUnvetted int
}

// observer observes on the metainfo loop and adds segments to node reservoirs
type observer struct {
	log *zap.Logger

	overlay         *overlay.Service
	reservoirs      map[storj.NodeID]*Reservoir
	reservoirConfig reservoirConfig
}

func newObserver(log *zap.Logger, overlay *overlay.Service, config reservoirConfig) *observer {
	return &observer{
		log:             log,
		overlay:         overlay,
		reservoirs:      make(map[storj.NodeID]*Reservoir),
		reservoirConfig: config,
	}
}

// RemoteSegment takes a remote segment found in metainfo and creates a reservoir for it if it doesn't exist already
func (observer *observer) RemoteSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx, path)(&err)

	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		if _, ok := observer.reservoirs[piece.NodeId]; !ok {
			reputable, err := observer.overlay.IsVetted(ctx, piece.NodeId)
			if err != nil {
				observer.log.Error("error finding if node is vetted", zap.Error(err))
				return nil
			}
			var slots int
			if reputable {
				slots = observer.reservoirConfig.slotsForVetted
			} else {
				slots = observer.reservoirConfig.slotsForUnvetted
			}
			observer.reservoirs[piece.NodeId] = NewReservoir(slots)
		}
		observer.reservoirs[piece.NodeId].sample(path)
	}
	return nil
}

// RemoteObject returns nil because the audit service does not interact with remote objects
func (observer *observer) RemoteObject(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	return nil
}

// InlineSegment returns nil because we're only auditing for storage nodes for now
func (observer *observer) InlineSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	return nil
}
