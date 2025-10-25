// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package nodetally

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
)

var (
	// Error is a standard error class for this package.
	Error = errs.Class("node tally")
	mon   = monkit.Package()
)

var (
	// check if Observer and Partial interfaces are satisfied.
	_ rangedloop.Observer = (*Observer)(nil)
	_ rangedloop.Partial  = (*observerFork)(nil)
)

// Config contains configurable values for nodetally observer.
type Config struct {
	BatchSize int `help:"batch size for saving tallies into DB" default:"1000" testDefault:"10"`
}

// Observer implements node tally ranged loop observer.
type Observer struct {
	log        *zap.Logger
	accounting accounting.StoragenodeAccounting

	metabaseDB *metabase.DB

	batchSize     int
	nowFn         func() time.Time
	lastTallyTime time.Time
	Node          map[metabase.NodeAlias]float64
}

// NewObserver creates new tally range loop observer.
func NewObserver(log *zap.Logger, accounting accounting.StoragenodeAccounting, metabaseDB *metabase.DB, config Config) *Observer {
	return &Observer{
		log:        log,
		accounting: accounting,
		metabaseDB: metabaseDB,
		batchSize:  config.BatchSize,
		nowFn:      time.Now,
		Node:       map[metabase.NodeAlias]float64{},
	}
}

// Start implements ranged loop observer start method.
func (observer *Observer) Start(ctx context.Context, time time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	observer.Node = map[metabase.NodeAlias]float64{}
	observer.lastTallyTime, err = observer.accounting.LastTimestamp(ctx, accounting.LastAtRestTally)
	if err != nil {
		return err
	}
	if observer.lastTallyTime.IsZero() {
		observer.lastTallyTime = observer.nowFn()
	}
	return nil
}

// Fork forks new node tally ranged loop partial.
func (observer *Observer) Fork(ctx context.Context) (_ rangedloop.Partial, err error) {
	defer mon.Task()(&ctx)(&err)

	return newObserverFork(observer.log, observer.nowFn), nil
}

// Join joins node tally ranged loop partial to main observer updating main per node usage map.
func (observer *Observer) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer mon.Task()(&ctx)(&err)

	tallyPartial, ok := partial.(*observerFork)
	if !ok {
		return Error.New("expected partial type %T but got %T", tallyPartial, partial)
	}

	for alias, val := range tallyPartial.Node {
		observer.Node[alias] += val
	}

	return nil
}

// for backwards compatibility.
var monRangedTally = monkit.ScopeNamed("storj.io/storj/satellite/accounting/tally")

// Finish calculates byte*hours from per node storage usage and save tallies to DB.
func (observer *Observer) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	finishTime := observer.nowFn()

	// calculate byte hours, not just bytes
	hours := finishTime.Sub(observer.lastTallyTime).Hours()
	var totalSum float64
	nodeIDs := make([]storj.NodeID, 0, observer.batchSize)
	byteHours := make([]float64, 0, observer.batchSize)
	nodeAliasMap, err := observer.metabaseDB.LatestNodesAliasMap(ctx)
	if err != nil {
		return err
	}

	var errs errs.Group
	for alias, pieceSize := range observer.Node {
		totalSum += pieceSize
		nodeID, ok := nodeAliasMap.Node(alias)
		if !ok {
			observer.log.Error("unrecognized node alias in ranged-loop tally", zap.Int32("node-alias", int32(alias)))
			continue
		}

		nodeIDs = append(nodeIDs, nodeID)
		byteHours = append(byteHours, pieceSize*hours)

		if len(nodeIDs) >= observer.batchSize {
			err = observer.accounting.SaveTallies(ctx, finishTime, nodeIDs, byteHours)
			if err != nil {
				errs.Add(Error.New("StorageNodeAccounting.SaveTallies failed: %v", err))
			}

			nodeIDs = nodeIDs[:0]
			byteHours = byteHours[:0]
		}
	}

	monRangedTally.IntVal("nodetallies.totalsum").Observe(int64(totalSum))

	err = observer.accounting.SaveTallies(ctx, finishTime, nodeIDs, byteHours)
	if err != nil {
		errs.Add(Error.New("StorageNodeAccounting.SaveTallies failed: %v", err))
	}

	return errs.Err()
}

// SetNow overrides the timestamp used to store the result.
func (observer *Observer) SetNow(nowFn func() time.Time) {
	observer.nowFn = nowFn
}

// observerFork implements node tally ranged loop partial.
type observerFork struct {
	log   *zap.Logger
	nowFn func() time.Time

	Node map[metabase.NodeAlias]float64
}

// newObserverFork creates new node tally ranged loop fork.
func newObserverFork(log *zap.Logger, nowFn func() time.Time) *observerFork {
	return &observerFork{
		log:   log,
		nowFn: nowFn,
		Node:  map[metabase.NodeAlias]float64{},
	}
}

// Process iterates over segment range updating partial node usage map.
func (partial *observerFork) Process(ctx context.Context, segments []rangedloop.Segment) error {
	now := partial.nowFn()

	for _, segment := range segments {
		partial.processSegment(now, segment)
	}

	return nil
}

func (partial *observerFork) processSegment(now time.Time, segment rangedloop.Segment) {
	if segment.Inline() {
		return
	}

	if segment.Expired(now) {
		return
	}

	// add node info
	minimumRequired := segment.Redundancy.RequiredShares

	if minimumRequired <= 0 {
		partial.log.Error("failed sanity check", zap.String("StreamID", segment.StreamID.String()), zap.Uint64("Position", segment.Position.Encode()))
		return
	}

	pieceSize := float64(segment.PieceSize())
	for _, piece := range segment.AliasPieces {
		partial.Node[piece.Alias] += pieceSize
	}
}
