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
	"storj.io/storj/pkg/storj"
)

var (
	mon = monkit.Package()

	// Error defines the piece tracker errors class
	Error = errs.Class("piece tracker error")
)

// RetainInfo contains info needed for a storage node to retain important data and delete garbage data
type RetainInfo struct {
	Filter       *bloomfilter.Filter
	CreationDate time.Time
	count        int
}

// PieceTracker allows access to info about the good pieces that storage nodes need to retain
type PieceTracker interface {
	// Add adds a RetainInfo to the PieceTracker
	Add(ctx context.Context, nodeID storj.NodeID, pieceID storj.PieceID) error
	// GetRetainInfos gets all of the RetainInfos
	GetRetainInfos() map[storj.NodeID]*RetainInfo
}

// pieceTracker contains info about the good pieces that storage nodes need to retain
type pieceTracker struct {
	log                *zap.Logger
	filterCreationDate time.Time
	initialPieces      int64
	falsePositiveRate  float64
	Requests           map[storj.NodeID]*RetainInfo
	pieceCounts        map[storj.NodeID]int
}

// Add adds a pieceID to the relevant node's RetainInfo
func (pieceTracker *pieceTracker) Add(ctx context.Context, nodeID storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	var filter *bloomfilter.Filter

	// if we know how many pieces a node should be storing, use that number. Otherwise use default
	numPieces := int(pieceTracker.initialPieces)
	if pieceTracker.pieceCounts[nodeID] > 0 {
		numPieces = pieceTracker.pieceCounts[nodeID]
	}
	if _, ok := pieceTracker.Requests[nodeID]; !ok {
		filter = bloomfilter.NewOptimal(numPieces, pieceTracker.falsePositiveRate)
		pieceTracker.Requests[nodeID].Filter = filter
		pieceTracker.Requests[nodeID].CreationDate = pieceTracker.filterCreationDate
	}

	pieceTracker.Requests[nodeID].Filter.Add(pieceID)
	pieceTracker.Requests[nodeID].count++
	return nil
}

// GetRetainInfos returns the retain requests on the pieceTracker struct
func (pieceTracker *pieceTracker) GetRetainInfos() map[storj.NodeID]*RetainInfo {
	return pieceTracker.Requests
}

// noOpPieceTracker does nothing when PieceTracker methods are called, because it's not time for the next iteration.
type noOpPieceTracker struct {
}

// Add adds nothing when using the noOpPieceTracker
func (pieceTracker *noOpPieceTracker) Add(ctx context.Context, nodeID storj.NodeID, pieceID storj.PieceID) (err error) {
	return nil
}

// GetRetainInfos returns nothing when using the noOpPieceTracker
func (pieceTracker *noOpPieceTracker) GetRetainInfos() map[storj.NodeID]*RetainInfo {
	return nil
}
