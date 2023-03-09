// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/segmentloop"
)

var remoteSegmentFunc = mon.Task()

var _ segmentloop.Observer = (*PathCollector)(nil)
var _ rangedloop.Partial = (*PathCollector)(nil)

// PathCollector uses the metainfo loop to add paths to node reservoirs.
//
// architecture: Observer
type PathCollector struct {
	log           *zap.Logger
	db            DB
	buffer        []TransferQueueItem
	batchSize     int
	nodeIDStorage map[storj.NodeID]int64
}

// NewPathCollector instantiates a path collector.
func NewPathCollector(log *zap.Logger, db DB, exitingNodes storj.NodeIDList, batchSize int) *PathCollector {
	collector := &PathCollector{
		log:           log,
		db:            db,
		buffer:        make([]TransferQueueItem, 0, batchSize),
		batchSize:     batchSize,
		nodeIDStorage: make(map[storj.NodeID]int64, len(exitingNodes)),
	}

	if len(exitingNodes) > 0 {
		for _, nodeID := range exitingNodes {
			collector.nodeIDStorage[nodeID] = 0
		}
	}

	return collector
}

// LoopStarted is called at each start of a loop.
func (collector *PathCollector) LoopStarted(context.Context, segmentloop.LoopInfo) (err error) {
	return nil
}

// Flush persists the current buffer items to the database.
func (collector *PathCollector) Flush(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return collector.flush(ctx, 1)
}

// RemoteSegment takes a remote segment found in metainfo and creates a graceful exit transfer queue item if it doesn't exist already.
func (collector *PathCollector) RemoteSegment(ctx context.Context, segment *segmentloop.Segment) (err error) {
	defer remoteSegmentFunc(&ctx)(&err)
	if len(collector.nodeIDStorage) == 0 {
		return nil
	}
	return collector.handleRemoteSegment(ctx, segment)
}

func (collector *PathCollector) handleRemoteSegment(ctx context.Context, segment *segmentloop.Segment) (err error) {
	numPieces := len(segment.Pieces)
	for _, piece := range segment.Pieces {
		if _, ok := collector.nodeIDStorage[piece.StorageNode]; !ok {
			continue
		}

		pieceSize := segment.PieceSize()

		collector.nodeIDStorage[piece.StorageNode] += pieceSize

		item := TransferQueueItem{
			NodeID:          piece.StorageNode,
			StreamID:        segment.StreamID,
			Position:        segment.Position,
			PieceNum:        int32(piece.Number),
			RootPieceID:     segment.RootPieceID,
			DurabilityRatio: float64(numPieces) / float64(segment.Redundancy.TotalShares),
		}

		collector.log.Debug("adding piece to transfer queue.", zap.Stringer("Node ID", piece.StorageNode),
			zap.String("stream_id", segment.StreamID.String()), zap.Int32("part", int32(segment.Position.Part)),
			zap.Int32("index", int32(segment.Position.Index)), zap.Uint16("piece num", piece.Number),
			zap.Int("num pieces", numPieces), zap.Int16("total possible pieces", segment.Redundancy.TotalShares))

		collector.buffer = append(collector.buffer, item)
		err = collector.flush(ctx, collector.batchSize)
		if err != nil {
			return err
		}
	}

	return nil
}

// InlineSegment returns nil because we're only auditing for storage nodes for now.
func (collector *PathCollector) InlineSegment(ctx context.Context, segment *segmentloop.Segment) (err error) {
	return nil
}

// Process adds transfer queue items for remote segments belonging to newly
// exiting nodes.
func (collector *PathCollector) Process(ctx context.Context, segments []segmentloop.Segment) (err error) {
	// Intentionally omitting mon.Task here. The duration for all process
	// calls are aggregated and and emitted by the ranged loop service.

	if len(collector.nodeIDStorage) == 0 {
		return nil
	}

	for _, segment := range segments {
		if segment.Inline() {
			continue
		}
		if err := collector.handleRemoteSegment(ctx, &segment); err != nil {
			return err
		}
	}
	return nil
}

func (collector *PathCollector) flush(ctx context.Context, limit int) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(collector.buffer) >= limit {
		err = collector.db.Enqueue(ctx, collector.buffer, collector.batchSize)
		collector.buffer = collector.buffer[:0]

		return errs.Wrap(err)
	}
	return nil
}
