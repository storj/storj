// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/bloomfilter"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/uplink/piecestore"
)

var (
	// Error defines the gc service errors class
	Error = errs.Class("gc service error")
	mon   = monkit.Package()
)

// Config contains configurable values for garbage collection
type Config struct {
	Interval time.Duration `help:"the time between each send of garbage collection filters to storage nodes" releaseDefault:"120h" devDefault:"10m"`
	Enabled  bool          `help:"set if garbage collection is enabled or not" releaseDefault:"true" devDefault:"true"`
	// value for InitialPieces currently based on average pieces per node
	InitialPieces     int     `help:"the initial number of pieces expected for a storage node to have, used for creating a filter" releaseDefault:"400000" devDefault:"10"`
	FalsePositiveRate float64 `help:"the false positive rate used for creating a garbage collection bloom filter" releaseDefault:"0.1" devDefault:"0.1"`
	ConcurrentSends   int     `help:"the number of nodes to concurrently send garbage collection bloom filters to" releaseDefault:"1" devDefault:"1"`
}

// Service implements the garbage collection service
//
// architecture: Chore
type Service struct {
	log    *zap.Logger
	config Config
	Loop   sync2.Cycle

	dialer       rpc.Dialer
	overlay      overlay.DB
	metainfoLoop *metainfo.Loop
}

// RetainInfo contains info needed for a storage node to retain important data and delete garbage data
type RetainInfo struct {
	Filter       *bloomfilter.Filter
	CreationDate time.Time
	Count        int
}

// NewService creates a new instance of the gc service
func NewService(log *zap.Logger, config Config, dialer rpc.Dialer, overlay overlay.DB, loop *metainfo.Loop) *Service {
	return &Service{
		log:    log,
		config: config,
		Loop:   *sync2.NewCycle(config.Interval),

		dialer:       dialer,
		overlay:      overlay,
		metainfoLoop: loop,
	}
}

// Run starts the gc loop service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !service.config.Enabled {
		return nil
	}

	// load last piece counts from overlay db
	lastPieceCounts, err := service.overlay.AllPieceCounts(ctx)
	if err != nil {
		service.log.Error("error getting last piece counts", zap.Error(err))
	}
	if lastPieceCounts == nil {
		lastPieceCounts = make(map[storj.NodeID]int)
	}

	return service.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		pieceTracker := NewPieceTracker(service.log.Named("gc observer"), service.config, lastPieceCounts)

		// collect things to retain
		err = service.metainfoLoop.Join(ctx, pieceTracker)
		if err != nil {
			service.log.Error("error joining metainfoloop", zap.Error(err))
			return nil
		}

		// save piece counts in memory for next iteration
		for id := range lastPieceCounts {
			delete(lastPieceCounts, id)
		}
		for id, info := range pieceTracker.retainInfos {
			lastPieceCounts[id] = info.Count
		}

		// save piece counts to db for next satellite restart
		err = service.overlay.UpdatePieceCounts(ctx, lastPieceCounts)
		if err != nil {
			service.log.Error("error updating piece counts", zap.Error(err))
		}

		// monitor information
		for _, info := range pieceTracker.retainInfos {
			mon.IntVal("node_piece_count").Observe(int64(info.Count))
			mon.IntVal("retain_filter_size_bytes").Observe(info.Filter.Size())
		}

		// send retain requests
		limiter := sync2.NewLimiter(service.config.ConcurrentSends)
		for id, info := range pieceTracker.retainInfos {
			id, info := id, info
			limiter.Go(ctx, func() {
				err := service.sendRetainRequest(ctx, id, info)
				if err != nil {
					service.log.Error("error sending retain info to node", zap.Stringer("Node ID", id), zap.Error(err))
				}
			})
		}
		limiter.Wait()

		return nil
	})
}

func (service *Service) sendRetainRequest(ctx context.Context, id storj.NodeID, info *RetainInfo) (err error) {
	defer mon.Task()(&ctx, id.String())(&err)

	log := service.log.Named(id.String())

	dossier, err := service.overlay.Get(ctx, id)
	if err != nil {
		return Error.Wrap(err)
	}

	client, err := piecestore.Dial(ctx, service.dialer, &dossier.Node, log, piecestore.DefaultConfig)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, Error.Wrap(client.Close()))
	}()

	err = client.Retain(ctx, &pb.RetainRequest{
		CreationDate: info.CreationDate,
		Filter:       info.Filter.Bytes(),
	})
	return Error.Wrap(err)
}
