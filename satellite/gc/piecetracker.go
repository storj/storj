// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/bloomfilter"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metainfo"
)

var _ metainfo.Observer = (*PieceTracker)(nil)

// PieceTracker implements the metainfo loop observer interface for garbage collection
//
// architecture: Observer
type PieceTracker struct {
	log          *zap.Logger
	config       Config
	creationDate time.Time
	// TODO: should we use int or int64 consistently for piece count (db type is int64)?
	pieceCounts map[storj.NodeID]int

	retainInfos map[storj.NodeID]*RetainInfo
}

// NewPieceTracker instantiates a new gc piece tracker to be subscribed to the metainfo loop
func NewPieceTracker(log *zap.Logger, config Config, pieceCounts map[storj.NodeID]int) *PieceTracker {
	return &PieceTracker{
		log:          log,
		config:       config,
		creationDate: time.Now().UTC(),
		pieceCounts:  pieceCounts,

		retainInfos: make(map[storj.NodeID]*RetainInfo),
	}
}

// RemoteSegment takes a remote segment found in metainfo and adds pieces to bloom filters
func (pieceTracker *PieceTracker) RemoteSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx, path.Raw)(&err)

	remote := pointer.GetRemote()
	pieces := remote.GetRemotePieces()

	for _, piece := range pieces {
		pieceID := remote.RootPieceId.Derive(piece.NodeId, piece.PieceNum)
		pieceTracker.add(piece.NodeId, pieceID)
	}
	return nil
}

// Object returns nil because gc does not interact with remote objects
func (pieceTracker *PieceTracker) Object(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return nil
}

// InlineSegment returns nil because we're only doing gc for storage nodes for now
func (pieceTracker *PieceTracker) InlineSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return nil
}

// adds a pieceID to the relevant node's RetainInfo
func (pieceTracker *PieceTracker) add(nodeID storj.NodeID, pieceID storj.PieceID) {
	if _, ok := pieceTracker.retainInfos[nodeID]; !ok {
		// If we know how many pieces a node should be storing, use that number. Otherwise use default.
		numPieces := pieceTracker.config.InitialPieces
		if pieceTracker.pieceCounts[nodeID] > 0 {
			numPieces = pieceTracker.pieceCounts[nodeID]
		}
		// limit size of bloom filter to ensure we are under the limit for GRPC
		filter := bloomfilter.NewOptimalMaxSize(numPieces, pieceTracker.config.FalsePositiveRate, 2*memory.MiB)
		pieceTracker.retainInfos[nodeID] = &RetainInfo{
			Filter:       filter,
			CreationDate: pieceTracker.creationDate,
		}
	}

	pieceTracker.retainInfos[nodeID].Filter.Add(pieceID)
	pieceTracker.retainInfos[nodeID].Count++
}
