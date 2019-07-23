// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"sync"
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
	Interval time.Duration `help:"the time between each send of garbage collection filters to storage nodes" releaseDefault:"168h" devDefault:"10m"`
	Enabled  bool          `help:"set if garbage collection is enabled or not" releaseDefault:"true" devDefault:"true"`
	// value for InitialPieces currently based on average pieces per node
	InitialPieces     int64   `help:"the initial number of pieces expected for a storage node to have, used for creating a filter" releaseDefault:"400000" devDefault:"10"`
	FalsePositiveRate float64 `help:"the false positive rate used for creating a filter" releaseDefault:"0.1" devDefault:"0.1"`
	ConcurrentSends   int64   `help:"the number of nodes to concurrently send bloom filters to" releaseDefault:"1" devDefault:"1"`
}

// Service implements the garbage collection service
type Service struct {
	log          *zap.Logger
	Loop         *sync2.Cycle
	metainfoloop *metainfo.Loop
	transport    transport.Client
	overlay      overlay.DB
	config       Config

	lastPieceCounts map[storj.NodeID]int

	pieceCountsMutex sync.Mutex
	pieceCounts      map[storj.NodeID]int
}

// RetainInfo contains info needed for a storage node to retain important data and delete garbage data
type RetainInfo struct {
	Filter       *bloomfilter.Filter
	CreationDate time.Time
	count        int
}

// NewService creates a new instance of the gc service
func NewService(log *zap.Logger, transport transport.Client, overlay overlay.DB, loop *metainfo.Loop, config Config) *Service {
	// TODO retrieve piece counts from overlay (when there is a column for them)
	// var lastPieceCounts atomic.Value
	// lastPieceCounts.Store(map[storj.NodeID]int{})
	lastPieceCounts := make(map[storj.NodeID]int)

	return &Service{
		log:              log,
		Loop:             sync2.NewCycle(config.Interval),
		metainfoloop:     loop,
		transport:        transport,
		overlay:          overlay,
		lastPieceCounts:  lastPieceCounts,
		config:           config,
		pieceCountsMutex: sync.Mutex{},
	}
}

// Run starts the gc loop service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		obs := NewObserver(service.log.Named("gc observer"), service.lastPieceCounts, service.config)

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

	service.pieceCounts = make(map[storj.NodeID]int)

	limiter := sync2.NewLimiter(int(service.config.ConcurrentSends))
	for id, retainInfo := range obs.retainInfos {
		service.sendRetainFromLimiter(ctx, id, retainInfo, limiter)
	}
	limiter.Wait()

	service.lastPieceCounts = service.pieceCounts

	return nil
}

func (service *Service) sendRetainFromLimiter(ctx context.Context, id storj.NodeID, retainInfo *RetainInfo, limiter *sync2.Limiter) {
	limiter.Go(ctx, func() {
		err := service.sendRetainRequest(ctx, id, retainInfo)
		if err != nil {
			service.log.Error("error sending retain info", zap.Error(err))
		}
	})
}

func (service *Service) sendRetainRequest(
	ctx context.Context, id storj.NodeID, retainInfo *RetainInfo,
) (err error) {
	defer mon.Task()(&ctx)(&err)

	log := service.log.Named(id.String())

	dossier, err := service.overlay.Get(ctx, id)
	if err != nil {
		return Error.Wrap(err)
	}

	target := &pb.Node{
		Id:      id,
		Address: dossier.Address,
	}

	ps, err := piecestore.Dial(ctx, service.transport, target, log, piecestore.DefaultConfig)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() {
		err2 := ps.Close()
		err = errs.Combine(err, Error.Wrap(err2))
	}()

	// TODO add piece count to overlay (when there is a column for them)
	service.pieceCountsMutex.Lock()
	service.pieceCounts[id] = retainInfo.count // save count for next bloom filter generation
	service.pieceCountsMutex.Unlock()
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
