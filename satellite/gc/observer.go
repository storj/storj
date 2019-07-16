// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"time"

	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Observer implements the observer interface for gc
type Observer struct {
	pieceCounts  map[storj.NodeID]int
	retainInfos  map[storj.NodeID]*RetainInfo
	config       Config
	creationDate time.Time
}

// NewObserver instantiates a gc Observer
func NewObserver(pieceCounts map[storj.NodeID]int, config Config) *Observer {
	return &Observer{
		pieceCounts:  pieceCounts,
		retainInfos:  make(map[storj.NodeID]*RetainInfo),
		config:       config,
		creationDate: time.Now().UTC(),
	}
}

// RemoteSegment takes a remote segment found in metainfo and adds pieces to bloom filters
func (observer *Observer) RemoteSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	return nil
}

// RemoteObject returns nil because gc does not interact with remote objects
func (observer *Observer) RemoteObject(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

// InlineSegment returns nil because we're only doing gc for storage nodes for now
func (observer *Observer) InlineSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

// adds a pieceID to the relevant node's RetainInfo
func (observer *Observer) add(ctx context.Context, nodeID storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	var filter *bloomfilter.Filter

	if _, ok := observer.retainInfos[nodeID]; !ok {
		// If we know how many pieces a node should be storing, use that number. Otherwise use default.
		// todo set default from config`j
		numPieces := int(400000)
		if observer.pieceCounts[nodeID] > 0 {
			numPieces = observer.pieceCounts[nodeID]
		}
		filter = bloomfilter.NewOptimal(numPieces, observer.config.FalsePositiveRate)
		observer.retainInfos[nodeID] = &RetainInfo{
			Filter:       filter,
			CreationDate: observer.creationDate,
		}
	}

	observer.retainInfos[nodeID].Filter.Add(pieceID)
	observer.retainInfos[nodeID].count++
	return nil
}
