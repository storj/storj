// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/uplink/piecestore"
)

// Config contains configurable values for garbage collection
type Config struct {
	Interval time.Duration `help:"how frequently garbage collection filters should be sent to storage nodes" releaseDefault:"168h" devDefault:"10m"`
	Active   bool          `help:"set if garbage collection is actively running or not" releaseDefault:"true" devDefault:"true"`
	// value for InitialPieces currently based on average pieces per node
	InitialPieces     int64   `help:"the initial number of pieces expected for a storage node to have, used for creating a filter" releaseDefault:"400000" devDefault:"10"`
	FalsePositiveRate float64 `help:"the false positive rate used for creating a filter" releaseDefault:"0.1" devDefault:"0.1"`
}

// Service implements the garbage collection service
type Service struct {
	log             *zap.Logger
	config          Config
	transport       transport.Client
	lastSendTime    time.Time
	lastPieceCounts map[storj.NodeID]int
}

// NewService creates a new instance of the gc service
func NewService(log *zap.Logger, config Config, transport transport.Client) *Service {
	return &Service{
		log:             log,
		transport:       transport,
		config:          config,
		lastPieceCounts: make(map[storj.NodeID]int),
	}
}

// NewPieceTracker instantiates a piece tracker
func (service *Service) NewPieceTracker() PieceTracker {
	// Creation date of the gc bloom filter - the storage nodes shouldn't delete any piece newer than this.
	filterCreationDate := time.Now().UTC()

	if !service.config.Active || filterCreationDate.Before(service.lastSendTime.Add(service.config.Interval)) {
		return &noOpPieceTracker{}
	}

	return &pieceTracker{
		log:                service.log.Named("piecetracker"),
		filterCreationDate: filterCreationDate,
		initialPieces:      service.config.InitialPieces,
		falsePositiveRate:  service.config.FalsePositiveRate,
		retainInfos:        make(map[storj.NodeID]*RetainInfo),
		pieceCounts:        service.lastPieceCounts,
	}
}

// Send sends the piece retain requests to all storage nodes
func (service *Service) Send(ctx context.Context, pieceTracker PieceTracker) (err error) {
	defer mon.Task()(&ctx)(&err)

	service.lastPieceCounts = make(map[storj.NodeID]int)
	service.lastSendTime = time.Now().UTC()

	for id, retainInfo := range pieceTracker.GetRetainInfos() {
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

		service.lastPieceCounts[id] = retainInfo.count // save count for next bloom filter generation
		mon.IntVal("node_piece_count").Observe(int64(retainInfo.count))

		filterBytes := retainInfo.Filter.Bytes()
		mon.IntVal("retain_filter_size_bytes").Observe(int64(len(filterBytes)))

		// TODO: send the retain request to the storage node (PR #2424)
		// retainReq := &pb.RetainRequest{
		// 	CreationDate: retainInfo.CreationDate,
		// 	Filter:       filterBytes,
		// }
		// _, err := ps.gcendpoint.Retain(ctx, retainReq)
		// if err != nil {
		// 	return Error.Wrap(err)
		// }
	}

	return nil
}
