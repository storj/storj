// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/uplink/piecestore"
)

var (
	// Error defines the gc service errors class
	Error = errs.Class("gc service error")
)

// Config contains configurable values for garbage collection
type Config struct {
	Interval time.Duration `help:"how frequently garbage collection filters should be sent to storage nodes" releaseDefault:"168h" devDefault:"10m"`
	Enabled  bool          `help:"set if garbage collection is enabled or not" releaseDefault:"true" devDefault:"true"`
	// value for InitialPieces currently based on average pieces per node
	InitialPieces     int64   `help:"the initial number of pieces expected for a storage node to have, used for creating a filter" releaseDefault:"400000" devDefault:"10"`
	FalsePositiveRate float64 `help:"the false positive rate used for creating a filter" releaseDefault:"0.1" devDefault:"0.1"`
}

// Service implements the garbage collection service
type Service struct {
	log             *zap.Logger
	config          Config
	transport       transport.Client
	overlay         overlay.DB
	lastSendTime    time.Time
	lastPieceCounts map[storj.NodeID]int
}

// NewService creates a new instance of the gc service
func NewService(log *zap.Logger, config Config, transport transport.Client, overlay overlay.DB) *Service {
	return &Service{
		log:             log,
		config:          config,
		transport:       transport,
		overlay:         overlay,
		lastPieceCounts: make(map[storj.NodeID]int),
	}
}

// NewPieceTracker instantiates a piece tracker
func (service *Service) NewPieceTracker() *PieceTracker {
	// Creation date of the gc bloom filter - the storage nodes shouldn't delete any piece newer than this.
	filterCreationDate := time.Now().UTC()

	if !service.config.Enabled || filterCreationDate.Before(service.lastSendTime.Add(service.config.Interval)) {
		return nil
	}

	return &PieceTracker{
		log:                service.log.Named("piecetracker"),
		filterCreationDate: filterCreationDate,
		initialPieces:      service.config.InitialPieces,
		falsePositiveRate:  service.config.FalsePositiveRate,
		retainInfos:        make(map[storj.NodeID]*RetainInfo),
		pieceCounts:        service.lastPieceCounts,
		overlay:            service.overlay,
	}
}

// Send sends the piece retain requests to all storage nodes
func (service *Service) Send(ctx context.Context, pieceTracker *PieceTracker, cb func()) (err error) {
	defer mon.Task()(&ctx)(&err)

	service.lastSendTime = time.Now().UTC()

	go func() {
		err := service.sendRetainRequests(ctx, pieceTracker)
		if err != nil {
			service.log.Error("error sending retain infos", zap.Error(err))
		}
		cb()
	}()

	return nil
}

func (service *Service) sendRetainRequests(ctx context.Context, pieceTracker *PieceTracker) (err error) {
	defer mon.Task()(&ctx)(&err)

	var errList errs.Group
	for id, retainInfo := range pieceTracker.GetRetainInfos() {
		err := service.sendOneRetainRequest(ctx, id, retainInfo)
		if err != nil {
			errList.Add(err)
		}
	}
	return errList.Err()
}

func (service *Service) sendOneRetainRequest(ctx context.Context, id storj.NodeID, retainInfo *RetainInfo) (err error) {
	defer mon.Task()(&ctx)(&err)

	log := service.log.Named(id.String())

	target := &pb.Node{
		Id:      id,
		Address: retainInfo.address,
	}
	signer := signing.SignerFromFullIdentity(service.transport.Identity())

	ps, err := piecestore.Dial(ctx, service.transport, target, log, signer, piecestore.DefaultConfig)
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, ps.Close())
	}()

	service.lastPieceCounts[id] = retainInfo.count // save count for next bloom filter generation
	mon.IntVal("node_piece_count").Observe(int64(retainInfo.count))

	filterBytes := retainInfo.Filter.Bytes()
	mon.IntVal("retain_filter_size_bytes").Observe(int64(len(filterBytes)))

	retainReq := &pb.RetainRequest{
		CreationDate: retainInfo.CreationDate,
		Filter:       filterBytes,
	}
	return ps.Retain(ctx, retainReq)
}
