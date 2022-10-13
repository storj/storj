// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

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

var _ segmentloop.Observer = (*PieceTracker)(nil)

// RetainInfo contains info needed for a storage node to retain important data and delete garbage data.
type RetainInfo struct {
	Filter *bloomfilter.Filter
	Count  int
}

// PieceTracker implements the segments loop observer interface for garbage collection.
//
// architecture: Observer
type PieceTracker struct {
	log    *zap.Logger
	config Config
	// TODO: should we use int or int64 consistently for piece count (db type is int64)?
	pieceCounts map[storj.NodeID]int64
	startTime   time.Time

	RetainInfos map[storj.NodeID]*RetainInfo
	// LatestCreationTime will be used to set bloom filter CreationDate.
	// Because bloom filter service needs to be run against immutable database snapshot
	// we can set CreationDate for bloom filters as a latest segment CreatedAt value.
	LatestCreationTime time.Time
}

// NewPieceTracker instantiates a new gc piece tracker to be subscribed to the segments loop.
func NewPieceTracker(log *zap.Logger, config Config, pieceCounts map[storj.NodeID]int64) *PieceTracker {
	return &PieceTracker{
		log:         log,
		config:      config,
		pieceCounts: pieceCounts,

		RetainInfos: make(map[storj.NodeID]*RetainInfo, len(pieceCounts)),
	}
}

// LoopStarted is called at each start of a loop.
func (pieceTracker *PieceTracker) LoopStarted(ctx context.Context, info segmentloop.LoopInfo) (err error) {
	pieceTracker.startTime = info.Started
	return nil
}

// RemoteSegment takes a remote segment found in metabase and adds pieces to bloom filters.
func (pieceTracker *PieceTracker) RemoteSegment(ctx context.Context, segment *segmentloop.Segment) error {
	// we are expliticy not adding monitoring here as we are tracking loop observers separately

	// sanity check to detect if loop is not running against live database
	if segment.CreatedAt.After(pieceTracker.startTime) {
		pieceTracker.log.Error("segment created after loop started", zap.Stringer("StreamID", segment.StreamID),
			zap.Time("loop started", pieceTracker.startTime),
			zap.Time("segment created", segment.CreatedAt))
		return errs.New("segment created after loop started")
	}

	if pieceTracker.LatestCreationTime.Before(segment.CreatedAt) {
		pieceTracker.LatestCreationTime = segment.CreatedAt
	}

	deriver := segment.RootPieceID.Deriver()
	for _, piece := range segment.Pieces {
		pieceID := deriver.Derive(piece.StorageNode, int32(piece.Number))
		pieceTracker.add(piece.StorageNode, pieceID)
	}

	return nil
}

// add adds a pieceID to the relevant node's RetainInfo.
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
			Filter: filter,
		}
		pieceTracker.RetainInfos[nodeID] = info
	}

	info.Filter.Add(pieceID)
	info.Count++
}

// InlineSegment returns nil because we're only doing gc for storage nodes for now.
func (pieceTracker *PieceTracker) InlineSegment(ctx context.Context, segment *segmentloop.Segment) (err error) {
	return nil
}
