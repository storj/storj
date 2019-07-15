// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var (
	mon = monkit.Package()

	// PieceTrackerError defines the piece tracker errors class
	PieceTrackerError = errs.Class("piece tracker error")
)

// RetainInfo contains info needed for a storage node to retain important data and delete garbage data
type RetainInfo struct {
	Filter       *bloomfilter.Filter
	CreationDate time.Time
	address      *pb.NodeAddress
	count        int
}

// PieceTracker contains info about the existing pieces that storage nodes need to retain
type PieceTracker struct {
	log                *zap.Logger
	overlay            overlay.DB
	filterCreationDate time.Time
	initialPieces      int64
	falsePositiveRate  float64
	retainInfos        map[storj.NodeID]*RetainInfo

	// This map MUST ONLY BE USED as readonly
	pieceCounts map[storj.NodeID]int
}

// Add adds a pieceID to the relevant node's RetainInfo
func (pieceTracker *PieceTracker) Add(ctx context.Context, nodeID storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	var filter *bloomfilter.Filter

	if _, ok := pieceTracker.retainInfos[nodeID]; !ok {
		// If we know how many pieces a node should be storing, use that number. Otherwise use default.
		numPieces := int(pieceTracker.initialPieces)
		if pieceTracker.pieceCounts[nodeID] > 0 {
			numPieces = pieceTracker.pieceCounts[nodeID]
		}
		node, err := pieceTracker.overlay.Get(ctx, nodeID)
		if err != nil {
			return PieceTrackerError.Wrap(err)
		}
		address := node.GetAddress()
		filter = bloomfilter.NewOptimal(numPieces, pieceTracker.falsePositiveRate)
		pieceTracker.retainInfos[nodeID] = &RetainInfo{
			address:      address,
			Filter:       filter,
			CreationDate: pieceTracker.filterCreationDate,
		}
	}

	pieceTracker.retainInfos[nodeID].Filter.Add(pieceID)
	pieceTracker.retainInfos[nodeID].count++
	return nil
}

// GetRetainInfos returns the retain requests on the pieceTracker struct
func (pieceTracker *PieceTracker) GetRetainInfos() map[storj.NodeID]*RetainInfo {
	return pieceTracker.retainInfos
}
