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

// Observer implements the observer interface for gc
type Observer struct {
	pieceCounts  map[storj.NodeID]int
	retainInfos  map[storj.NodeID]*RetainInfo
	config       Config
	creationDate time.Time
}

// RetainInfo contains info needed for a storage node to retain important data and delete garbage data
type RetainInfo struct {
	Filter       *bloomfilter.Filter
	CreationDate time.Time
	address      *pb.NodeAddress
	count        int
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
		obs := NewObserver(pieceCounts, service.config)

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
		err := service.sendOneRetainRequest(ctx, id, retainInfo, newPieceCounts)
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

func (service *Service) sendOneRetainRequest(
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
		err = errs.Combine(err, ps.Close())
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
	return ps.Retain(ctx, retainReq)
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
