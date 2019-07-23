// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// PieceTracker implements the metainfo loop observer interface for garbage collection
type PieceTracker struct {
	log          *zap.Logger
	config       Config
	creationDate time.Time
	pieceCounts  map[storj.NodeID]int

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
func (pt *PieceTracker) RemoteSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	remote := pointer.GetRemote()
	pieces := remote.GetRemotePieces()

	for _, piece := range pieces {
		pieceID := remote.RootPieceId.Derive(piece.NodeId, piece.PieceNum)
		pt.add(ctx, piece.NodeId, pieceID)
	}
	return nil
}

// RemoteObject returns nil because gc does not interact with remote objects
func (pt *PieceTracker) RemoteObject(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

// InlineSegment returns nil because we're only doing gc for storage nodes for now
func (pt *PieceTracker) InlineSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

// adds a pieceID to the relevant node's RetainInfo
func (pt *PieceTracker) add(ctx context.Context, nodeID storj.NodeID, pieceID storj.PieceID) {
	var filter *bloomfilter.Filter

	if _, ok := pt.retainInfos[nodeID]; !ok {
		// If we know how many pieces a node should be storing, use that number. Otherwise use default.
		numPieces := pt.config.InitialPieces
		if pt.pieceCounts[nodeID] > 0 {
			numPieces = pt.pieceCounts[nodeID]
		}
		// limit size of bloom filter to ensure we are under the limit for GRPC
		filter = bloomfilter.NewOptimalMaxSize(numPieces, pt.config.FalsePositiveRate, 2*memory.MiB)
		pt.retainInfos[nodeID] = &RetainInfo{
			Filter:       filter,
			CreationDate: pt.creationDate,
		}
	}

	pt.retainInfos[nodeID].Filter.Add(pieceID)
	pt.retainInfos[nodeID].Count++
}
