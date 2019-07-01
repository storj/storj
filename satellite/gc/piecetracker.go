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

// pieceTracker contains info about the good pieces that storage nodes need to retain
type pieceTracker struct {
	log                *zap.Logger
	filterCreationDate time.Time
	initialPieces      int64
	falsePositiveRate  float64
	Requests           map[storj.NodeID]*RetainInfo
}

// noOpPieceTracker does nothing when PieceTracker methods are called, because it's not time for the next iteration.
type noOpPieceTracker struct {
}

// PieceTracker allows access to info about the good pieces that storage nodes need to retain
type PieceTracker interface {
	// Add adds a RetainInfo to the PieceTracker
	Add(ctx context.Context, nodeID storj.NodeID, pieceID storj.PieceID) error
	// GetRetainInfos gets all of the RetainInfos
	GetRetainInfos() map[storj.NodeID]*RetainInfo
}

// NewPieceTracker instantiates a piece tracker
func (service *Service) NewPieceTracker() PieceTracker {
	// Creation date of the gc bloom filter - the storage node shouldn't delete any piece newer than this.
	filterCreationDate := time.Now().UTC()

	if filterCreationDate.Before(service.lastSendTime.Add(service.config.Interval)) {
		return &noOpPieceTracker{}
	}

	return &pieceTracker{
		log:                service.log.Named("piecetracker"),
		filterCreationDate: filterCreationDate,
		initialPieces:      service.config.InitialPieces,
		falsePositiveRate:  service.config.FalsePositiveRate,
		Requests:           make(map[storj.NodeID]*RetainInfo),
	}
}

// Service implements the garbage collection service
type Service struct {
	log          *zap.Logger
	config       Config
	transport    transport.Client
	lastSendTime time.Time
}

// NewService creates a new instance of the gc service
func NewService(log *zap.Logger, config Config, transport transport.Client) *Service {
	return &Service{
		log:       log,
		transport: transport,
		config:    config,
	}
}

// Add adds a pieceID to the relevant node's RetainInfo
func (pieceTracker *pieceTracker) Add(ctx context.Context, nodeID storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	var filter *bloomfilter.Filter

	if _, ok := pieceTracker.Requests[nodeID]; !ok {
		filter = bloomfilter.NewOptimal(int(pieceTracker.initialPieces), pieceTracker.falsePositiveRate)
		pieceTracker.Requests[nodeID].Filter = filter
		pieceTracker.Requests[nodeID].CreationDate = pieceTracker.filterCreationDate
	}

	pieceTracker.Requests[nodeID].Filter.Add(pieceID)
	return nil
}

// Add adds nothing when using the noOpPieceTracker
func (pieceTracker *noOpPieceTracker) Add(ctx context.Context, nodeID storj.NodeID, pieceID storj.PieceID) (err error) {
	return nil
}

// GetRetainInfos returns nothing when using the noOpPieceTracker
func (pieceTracker *noOpPieceTracker) GetRetainInfos() map[storj.NodeID]*RetainInfo {
	return nil
}

// GetRetainInfos returns the retain requests on the pieceTracker struct
func (pieceTracker *pieceTracker) GetRetainInfos() map[storj.NodeID]*RetainInfo {
	return pieceTracker.Requests
}

// Send sends the piece retain requests to all storage nodes
func (service *Service) Send(ctx context.Context, pieceTracker PieceTracker) (err error) {
	defer mon.Task()(&ctx)(&err)

	for id := range pieceTracker.GetRetainInfos() {
		log := service.log.Named(id.String())
		// TODO: access storage node address to populate target (can probably save in retain info when checker is iterating)
		target := &pb.Node{Id: id}
		signer := signing.SignerFromFullIdentity(service.transport.Identity())

		ps, err := piecestore.Dial(ctx, service.transport, target, log, signer, piecestore.DefaultConfig)
		if err != nil {
			return Error.Wrap(err)
		}
		defer func() {
			err := ps.Close()
			if err != nil {
				service.log.Error("piece tracker failed to close conn to node: %+v", zap.Error(err))
			}
		}()
		// TODO: send the retain request to the storage node
	}

	return nil
}
