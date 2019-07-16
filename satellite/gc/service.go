// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/uplink/piecestore"
)

var (
	// Error defines the gc service errors class
	Error = errs.Class("gc service error")
	mon   = monkit.Package()
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
	loop            *sync2.Cycle
	metainfoloop    *metainfo.LoopService
	retainInfos     map[storj.NodeID]*RetainInfo
	pieceCounts     map[storj.NodeID]int
	transport       transport.Client
	overlay         overlay.DB
	lastSendTime    time.Time
	lastPieceCounts atomic.Value
	config          Config
}

// RetainInfo contains info needed for a storage node to retain important data and delete garbage data
type RetainInfo struct {
	Filter       *bloomfilter.Filter
	CreationDate time.Time
	address      *pb.NodeAddress
	count        int
}

// NewService creates a new instance of the gc service
func NewService(log *zap.Logger, transport transport.Client, overlay overlay.DB, loop *metainfo.LoopService, config Config) *Service {
	var lastPieceCounts atomic.Value
	lastPieceCounts.Store(map[storj.NodeID]int{})

	return &Service{
		log:             log,
		loop:            sync2.NewCycle(config.Interval),
		metainfoloop:    loop,
		transport:       transport,
		overlay:         overlay,
		lastPieceCounts: lastPieceCounts,
		config:          config,
	}
}

// Run starts the gc loop service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.loop.Run(ctx, func(ctx context.Context) error {
		pieceCounts := service.lastPieceCountsValue()
		obs := NewObserver(service.log.Named("gc observer"), pieceCounts, service.config)

		err := service.metainfoloop.Join(ctx, obs)
		if err != nil {
			return Error.Wrap(err)
		}

		err = service.Send(ctx, obs)
		if err != nil {
			return Error.Wrap(err)
		}

		return nil
	})
}

// Send sends the piece retain requests to all storage nodes
func (service *Service) Send(ctx context.Context, obs *Observer) (err error) {
	defer mon.Task()(&ctx)(&err)

	service.lastSendTime = time.Now().UTC()
	newPieceCounts := make(map[storj.NodeID]int)

	// TODO: add a sync limiter so we can send multiple bloom filters concurrently
	var errList errs.Group
	for id, retainInfo := range obs.retainInfos {
		err := service.sendRetainRequest(ctx, id, retainInfo, newPieceCounts)
		if err != nil {
			errList.Add(err)
		}
	}
	if errList.Err() != nil {
		service.log.Error("error sending retain infos", zap.Error(errList.Err()))
	}
	service.lastPieceCounts.Store(newPieceCounts)

	return nil
}

func (service *Service) sendRetainRequest(
	ctx context.Context, id storj.NodeID, retainInfo *RetainInfo, newPieceCounts map[storj.NodeID]int,
) (err error) {
	defer mon.Task()(&ctx)(&err)

	log := service.log.Named(id.String())

	// todo get address from overlay
	target := &pb.Node{
		Id:      id,
		Address: retainInfo.address,
	}

	ps, err := piecestore.Dial(ctx, service.transport, target, log, piecestore.DefaultConfig)
	if err != nil {
		return err
	}
	defer func() {
		err2 := ps.Close()
		err = errs.Combine(err, Error.Wrap(err2))
	}()

	// todo add piece count to overlay (when there is a column for them)
	newPieceCounts[id] = retainInfo.count // save count for next bloom filter generation
	mon.IntVal("node_piece_count").Observe(int64(retainInfo.count))

	filterBytes := retainInfo.Filter.Bytes()
	mon.IntVal("retain_filter_size_bytes").Observe(int64(len(filterBytes)))

	retainReq := &pb.RetainRequest{
		CreationDate: retainInfo.CreationDate,
		Filter:       filterBytes,
	}
	err = ps.Retain(ctx, retainReq)
	if err != nil {
		return Error.Wrap(err)
	}
	return nil
}

func (service *Service) lastPieceCountsValue() map[storj.NodeID]int {
	m := service.lastPieceCounts.Load()
	if m == nil {
		return nil
	}

	return m.(map[storj.NodeID]int)
}

func (service *Service) lastPieceCountsNumNodes() int {
	m := service.lastPieceCounts.Load()
	if m == nil {
		return 0
	}

	return len(m.(map[storj.NodeID]int))
}
