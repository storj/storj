// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/uplink/piecestore"
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
}

// PieceTracker contains info about the good pieces that storage nodes need to retain
type PieceTracker struct {
	log       *zap.Logger
	config    Config
	transport transport.Client
	Requests  map[storj.NodeID]*RetainInfo
}

// NewPieceTracker instantiates a piece tracker
func NewPieceTracker(log *zap.Logger, config Config, transport transport.Client) *PieceTracker {
	return &PieceTracker{
		log:       log,
		transport: transport,
		config:    config,
		Requests:  make(map[storj.NodeID]*RetainInfo),
	}
}

// Add adds a RetainRequest to the Garbage "queue"
func (pieceTracker *PieceTracker) Add(ctx context.Context, nodeID storj.NodeID, pieceID storj.PieceID, creationDate time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	var filter *bloomfilter.Filter

	if _, ok := pieceTracker.Requests[nodeID]; !ok {
		filter = bloomfilter.NewOptimal(int(pieceTracker.config.InitialPieces), pieceTracker.config.FalsePositiveRate)
		pieceTracker.Requests[nodeID].Filter = filter
		pieceTracker.Requests[nodeID].CreationDate = creationDate
	}

	pieceTracker.Requests[nodeID].Filter.Add(pieceID)
	return nil
}

// Send sends the garbage retain requests to all storage nodes
func (pieceTracker *PieceTracker) Send(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for id := range pieceTracker.Requests {
		log := pieceTracker.log.Named(id.String())
		// TODO: access storage node address to populate target
		target := &pb.Node{Id: id}
		signer := signing.SignerFromFullIdentity(pieceTracker.transport.Identity())

		ps, err := piecestore.Dial(ctx, pieceTracker.transport, target, log, signer, piecestore.DefaultConfig)
		if err != nil {
			return Error.Wrap(err)
		}
		defer func() {
			err := ps.Close()
			if err != nil {
				pieceTracker.log.Error("piece tracker failed to close conn to node: %+v", zap.Error(err))
			}
		}()
		// TODO: send the retain request to the storage node
	}

	return nil
}
