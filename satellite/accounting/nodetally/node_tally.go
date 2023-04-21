// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package nodetally

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/segmentloop"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("node tally")
	mon   = monkit.Package()
)

// Service is the tally service for data stored on each storage node.
//
// architecture: Chore
type Service struct {
	log  *zap.Logger
	Loop *sync2.Cycle

	segmentLoop             *segmentloop.Service
	storagenodeAccountingDB accounting.StoragenodeAccounting
	metabaseDB              *metabase.DB
	nowFn                   func() time.Time
}

// New creates a new node tally Service.
func New(log *zap.Logger, sdb accounting.StoragenodeAccounting, mdb *metabase.DB, loop *segmentloop.Service, interval time.Duration) *Service {
	return &Service{
		log:  log,
		Loop: sync2.NewCycle(interval),

		segmentLoop:             loop,
		storagenodeAccountingDB: sdb,
		metabaseDB:              mdb,
		nowFn:                   time.Now,
	}
}

// Run the node tally service loop.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		err := service.Tally(ctx)
		if err != nil {
			service.log.Error("node tally failed", zap.Error(err))
		}
		return nil
	})
}

// Close stops the service and releases any resources.
func (service *Service) Close() error {
	service.Loop.Close()
	return nil
}

// SetNow allows tests to have the Service act as if the current time is whatever
// they want. This avoids races and sleeping, making tests more reliable and efficient.
func (service *Service) SetNow(now func() time.Time) {
	service.nowFn = now
}

// for backwards compatibility.
var monTally = monkit.ScopeNamed("storj.io/storj/satellite/accounting/tally")

// Tally calculates data-at-rest usage once.
func (service *Service) Tally(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Fetch when the last node tally happened so we can roughly calculate the byte-hours.
	lastTime, err := service.storagenodeAccountingDB.LastTimestamp(ctx, accounting.LastAtRestTally)
	if err != nil {
		return Error.Wrap(err)
	}
	if lastTime.IsZero() {
		lastTime = service.nowFn()
	}

	// add up all nodes
	observer := NewObserver(service.log.Named("observer"), service.nowFn())
	err = service.segmentLoop.Join(ctx, observer)
	if err != nil {
		return Error.Wrap(err)
	}
	finishTime := service.nowFn()

	// calculate byte hours, not just bytes
	hours := time.Since(lastTime).Hours()
	var totalSum float64
	for id, pieceSize := range observer.Node {
		totalSum += pieceSize
		observer.Node[id] = pieceSize * hours
	}
	monTally.IntVal("nodetallies.totalsum").Observe(int64(totalSum)) //mon:locked

	if len(observer.Node) > 0 {
		nodeIDs := make([]storj.NodeID, 0, len(observer.Node))
		nodeTotals := make([]float64, 0, len(observer.Node))
		nodeAliasMap, err := service.metabaseDB.LatestNodesAliasMap(ctx)
		if err != nil {
			return Error.Wrap(err)
		}
		for nodeAlias, total := range observer.Node {
			nodeID, ok := nodeAliasMap.Node(nodeAlias)
			if !ok {
				observer.log.Error("unrecognized node alias in tally", zap.Int32("node-alias", int32(nodeAlias)))
				continue
			}
			nodeIDs = append(nodeIDs, nodeID)
			nodeTotals = append(nodeTotals, total)
		}
		err = service.storagenodeAccountingDB.SaveTallies(ctx, finishTime, nodeIDs, nodeTotals)
		if err != nil {
			return Error.New("StorageNodeAccounting.SaveTallies failed: %v", err)
		}
	}

	return nil
}

var _ segmentloop.Observer = (*Observer)(nil)

// Observer observes metainfo and adds up tallies for nodes and buckets.
type Observer struct {
	log *zap.Logger
	now time.Time

	Node map[metabase.NodeAlias]float64
}

// NewObserver returns an segment loop observer that adds up totals for nodes.
func NewObserver(log *zap.Logger, now time.Time) *Observer {
	return &Observer{
		log: log,
		now: now,

		Node: make(map[metabase.NodeAlias]float64),
	}
}

// LoopStarted is called at each start of a loop.
func (observer *Observer) LoopStarted(context.Context, segmentloop.LoopInfo) (err error) {
	return nil
}

// RemoteSegment is called for each remote segment.
func (observer *Observer) RemoteSegment(ctx context.Context, segment *segmentloop.Segment) error {
	// we are expliticy not adding monitoring here as we are tracking loop observers separately

	if segment.Expired(observer.now) {
		return nil
	}

	// add node info
	minimumRequired := segment.Redundancy.RequiredShares

	if minimumRequired <= 0 {
		observer.log.Error("failed sanity check", zap.String("StreamID", segment.StreamID.String()), zap.Uint64("Position", segment.Position.Encode()))
		return nil
	}

	pieceSize := float64(segment.EncryptedSize / int32(minimumRequired)) // TODO: Add this as a method to RedundancyScheme

	for _, piece := range segment.AliasPieces {
		observer.Node[piece.Alias] += pieceSize
	}

	return nil
}

// InlineSegment is called for each inline segment.
func (observer *Observer) InlineSegment(ctx context.Context, segment *segmentloop.Segment) (err error) {
	return nil
}
