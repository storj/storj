// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metainfo"
	"storj.io/uplink/private/eestream"
)

var _ metainfo.Observer = (*PathCollector)(nil)

// PathCollector uses the metainfo loop to add paths to node reservoirs
//
// architecture: Observer
type PathCollector struct {
	db            DB
	nodeIDMutex   sync.Mutex
	nodeIDStorage map[storj.NodeID]int64
	buffer        []TransferQueueItem
	log           *zap.Logger
	batchSize     int
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
		collector.nodeIDStorage = make(map[storj.NodeID]int64, len(nodeIDs))
		for _, nodeID := range nodeIDs {
			collector.nodeIDStorage[nodeID] = 0
		}
	}

	return collector
}

// Flush persists the current buffer items to the database.
func (collector *PathCollector) Flush(ctx context.Context) (err error) {
	return collector.flush(ctx, 1)
}

// RemoteSegment takes a remote segment found in metainfo and creates a graceful exit transfer queue item if it doesn't exist already.
func (collector *PathCollector) RemoteSegment(ctx context.Context, segment *metainfo.Segment) (err error) {
	if len(collector.nodeIDStorage) == 0 {
		return nil
	}

	collector.nodeIDMutex.Lock()
	defer collector.nodeIDMutex.Unlock()

	numPieces := len(segment.Pieces)
	key := segment.Location.Encode()
	for _, piece := range segment.Pieces {
		if _, ok := collector.nodeIDStorage[piece.StorageNode]; !ok {
			continue
		}
		redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
		if err != nil {
			return err
		}
		pieceSize := eestream.CalcPieceSize(int64(segment.DataSize), redundancy)
		collector.nodeIDStorage[piece.StorageNode] += pieceSize

		item := TransferQueueItem{
			NodeID:          piece.StorageNode,
			Key:             key,
			PieceNum:        int32(piece.Number),
			RootPieceID:     segment.RootPieceID,
			DurabilityRatio: float64(numPieces) / float64(segment.Redundancy.TotalShares),
		}

		collector.log.Debug("adding piece to transfer queue.", zap.Stringer("Node ID", piece.StorageNode),
			zap.ByteString("key", key), zap.Uint16("piece num", piece.Number),
			zap.Int("num pieces", numPieces), zap.Int16("total possible pieces", segment.Redundancy.TotalShares))

		collector.buffer = append(collector.buffer, item)
		err = collector.flush(ctx, collector.batchSize)
		if err != nil {
			return err
		}
	}

	return nil
}

// Object returns nil because the audit service does not interact with objects.
func (collector *PathCollector) Object(ctx context.Context, object *metainfo.Object) (err error) {
	return nil
}

// InlineSegment returns nil because we're only auditing for storage nodes for now.
func (collector *PathCollector) InlineSegment(ctx context.Context, segment *metainfo.Segment) (err error) {
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
