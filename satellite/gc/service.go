// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
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
	InitialPieces     int     `help:"the initial number of pieces expected for a storage node to have, used for creating a filter" releaseDefault:"400000" devDefault:"10"`
	FalsePositiveRate float64 `help:"the false positive rate used for creating a filter" releaseDefault:"0.1" devDefault:"0.1"`
	ConcurrentSends   int     `help:"the number of nodes to concurrently send bloom filters to" releaseDefault:"1" devDefault:"1"`
}

// Service implements the garbage collection service
type Service struct {
	log    *zap.Logger
	config Config
	Loop   sync2.Cycle

	transport    transport.Client
	overlay      overlay.DB
	metainfoloop *metainfo.Loop
}

// RetainInfo contains info needed for a storage node to retain important data and delete garbage data
type RetainInfo struct {
	Filter       *bloomfilter.Filter
	CreationDate time.Time
	Count        int
}

// NewService creates a new instance of the gc service
func NewService(log *zap.Logger, config Config, transport transport.Client, overlay overlay.DB, loop *metainfo.Loop) *Service {
	return &Service{
		log:    log,
		config: config,
		Loop:   *sync2.NewCycle(config.Interval),

		transport:    transport,
		overlay:      overlay,
		metainfoloop: loop,
	}
}

// Run starts the gc loop service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO retrieve piece counts from overlay (when there is a column for them)
	lastPieceCounts := make(map[storj.NodeID]int)

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		obs := NewObserver(service.log.Named("gc observer"), service.config, lastPieceCounts)

		// collect things to retain
		err := service.metainfoloop.Join(ctx, obs)
		if err != nil {
			return Error.Wrap(err)
		}

		// send retain requests
		limiter := sync2.NewLimiter(service.config.ConcurrentSends)
		for id, info := range obs.retainInfos {
			id, info := id, info
			limiter.Go(ctx, func() {
				err := service.sendRetainRequest(ctx, id, info)
				if err != nil {
					service.log.Error("error sending retain info", zap.Error(err))
				}
			})
		}
		limiter.Wait()

		// save piece counts for next iteration
		for id := range lastPieceCounts {
			delete(lastPieceCounts, id)
		}
		for id, info := range obs.retainInfos {
			lastPieceCounts[id] = info.Count
		}

		// monitor information
		for _, info := range obs.retainInfos {
			mon.IntVal("node_piece_count").Observe(int64(info.Count))
			mon.IntVal("retain_filter_size_bytes").Observe(info.Filter.Size())
		}
		return nil
	})
}

func (service *Service) sendRetainRequest(ctx context.Context, id storj.NodeID, info *RetainInfo) (err error) {
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

	client, err := piecestore.Dial(ctx, service.transport, target, log, piecestore.DefaultConfig)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() {
		closeErr := Error.Wrap(client.Close())
		err = errs.Combine(err, closeErr)
	}()

	err = client.Retain(ctx, &pb.RetainRequest{
		CreationDate: info.CreationDate,
		Filter:       info.Filter.Bytes(),
	})
	return Error.Wrap(err)
}
