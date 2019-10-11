// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
)

var _ metainfo.Observer = (*PathCollector)(nil)

// PathCollector uses the metainfo loop to add paths to node reservoirs
//
// architecture: Observer
type PathCollector struct {
	db        DB
	nodeIDs   map[storj.NodeID]struct{}
	buffer    []TransferQueueItem
	log       *zap.Logger
	batchSize int
}

// NewPathCollector instantiates a path collector.
func NewPathCollector(db DB, nodeIDs storj.NodeIDList, log *zap.Logger, batchSize int) *PathCollector {
	buffer := make([]TransferQueueItem, 0, batchSize)
	collector := &PathCollector{
		db:        db,
		log:       log,
		buffer:    buffer,
		batchSize: batchSize,
	}

	if len(nodeIDs) > 0 {
		collector.nodeIDs = make(map[storj.NodeID]struct{}, len(nodeIDs))
		for _, nodeID := range nodeIDs {
			collector.nodeIDs[nodeID] = struct{}{}
		}
	}

	return collector
}

// Flush persists the current buffer items to the database.
func (collector *PathCollector) Flush(ctx context.Context) (err error) {
	return collector.flush(ctx, 1)
}

// RemoteSegment takes a remote segment found in metainfo and creates a graceful exit transfer queue item if it doesn't exist already
func (collector *PathCollector) RemoteSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	if len(collector.nodeIDs) == 0 {
		return nil
	}

	numPieces := int32(len(pointer.GetRemote().GetRemotePieces()))
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		if _, ok := collector.nodeIDs[piece.NodeId]; !ok {
			continue
		}

		item := TransferQueueItem{
			NodeID:          piece.NodeId,
			Path:            []byte(path.Raw),
			PieceNum:        piece.PieceNum,
			DurabilityRatio: float64(numPieces / pointer.GetRemote().GetRedundancy().GetTotal()),
		}
		collector.log.Debug("adding piece to transfer queue.", zap.String("node ID", piece.NodeId.String()),
			zap.String("path", path.Raw), zap.Int32("piece num", piece.GetPieceNum()),
			zap.Int32("num pieces", numPieces), zap.Int32("total possible pieces", pointer.GetRemote().GetRedundancy().GetTotal()))

		collector.buffer = append(collector.buffer, item)
		err = collector.flush(ctx, collector.batchSize)
		if err != nil {
			return err
		}
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

func (collector *PathCollector) flush(ctx context.Context, limit int) (err error) {
	if len(collector.buffer) >= limit {
		err = collector.db.Enqueue(ctx, collector.buffer)
		collector.buffer = collector.buffer[:0]

		return errs.Wrap(err)
	}
	return nil
}
