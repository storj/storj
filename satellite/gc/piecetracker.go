// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/bloomfilter"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/segmentloop"
)

var remoteSegmentFunc = mon.Task()

var _ segmentloop.Observer = (*PieceTracker)(nil)

// PieceTracker implements the metainfo loop observer interface for garbage collection.
//
// architecture: Observer
type PieceTracker struct {
	log          *zap.Logger
	config       Config
	creationDate time.Time
	// TODO: should we use int or int64 consistently for piece count (db type is int64)?
	pieceCounts map[storj.NodeID]int

	RetainInfos map[storj.NodeID]*RetainInfo
}

// NewPieceTracker instantiates a new gc piece tracker to be subscribed to the metainfo loop.
func NewPieceTracker(log *zap.Logger, config Config, pieceCounts map[storj.NodeID]int) *PieceTracker {
	return &PieceTracker{
		log:          log,
		config:       config,
		creationDate: time.Now().UTC(),
		pieceCounts:  pieceCounts,

		RetainInfos: make(map[storj.NodeID]*RetainInfo, len(pieceCounts)),
	}
}

// LoopStarted is called at each start of a loop.
func (pieceTracker *PieceTracker) LoopStarted(ctx context.Context, info segmentloop.LoopInfo) (err error) {
	if pieceTracker.creationDate.After(info.Started) {
		return errs.New("Creation date after loop starting time.")
	}
	return nil
}

// RemoteSegment takes a remote segment found in metabase and adds pieces to bloom filters.
func (pieceTracker *PieceTracker) RemoteSegment(ctx context.Context, segment *segmentloop.Segment) error {
	defer remoteSegmentFunc(&ctx)(nil) // method always returns nil

	deriver := segment.RootPieceID.Deriver()
	for _, piece := range segment.Pieces {
		pieceID := deriver.Derive(piece.StorageNode, int32(piece.Number))
		pieceTracker.add(piece.StorageNode, pieceID)
	}

	return nil
}

// InlineSegment returns nil because we're only doing gc for storage nodes for now.
func (pieceTracker *PieceTracker) InlineSegment(ctx context.Context, segment *segmentloop.Segment) (err error) {
	return nil
}

// adds a pieceID to the relevant node's RetainInfo.
func (pieceTracker *PieceTracker) add(nodeID storj.NodeID, pieceID storj.PieceID) {
	info, ok := pieceTracker.RetainInfos[nodeID]
	if !ok {
		// If we know how many pieces a node should be storing, use that number. Otherwise use default.
		numPieces := pieceTracker.config.InitialPieces
		if pieceTracker.pieceCounts[nodeID] > 0 {
			numPieces = pieceTracker.pieceCounts[nodeID]
		}
		// limit size of bloom filter to ensure we are under the limit for RPC
		filter := bloomfilter.NewOptimalMaxSize(numPieces, pieceTracker.config.FalsePositiveRate, 2*memory.MiB)
		info = &RetainInfo{
			Filter:       filter,
			CreationDate: pieceTracker.creationDate,
		}
		pieceTracker.RetainInfos[nodeID] = info
	}

	info.Filter.Add(pieceID)
	info.Count++
}
