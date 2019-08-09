package audit

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/overlay"
)

// auditObserver observes on the metainfo loop and adds segments to node reservoirs
type auditObserver struct {
	log *zap.Logger

	overlay         *overlay.Service
	Reservoirs      map[storj.NodeID]*Reservoir
	reservoirConfig reservoirConfig
}

type reservoirConfig struct {
	slotsForVetted   int
	slotsForUnvetted int
}

// RemoteSegment takes a remote segment found in metainfo and creates a reservoir for it if it doesn't exist already
func (observer *auditObserver) RemoteSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx, path)(&err)

	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		if _, ok := observer.Reservoirs[piece.NodeId]; !ok {
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
			observer.Reservoirs[piece.NodeId] = NewReservoir(slots)
		}
		observer.Reservoirs[piece.NodeId].sample(path)
	}
	return nil
}
