// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package nodetally

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/segmentloop"
)

var (
	// check if Observer and Partial interfaces are satisfied.
	_ rangedloop.Observer = (*RangedLoopObserver)(nil)
	_ rangedloop.Partial  = (*RangedLoopPartial)(nil)
)

// RangedLoopObserver implements node tally ranged loop observer.
type RangedLoopObserver struct {
	log        *zap.Logger
	accounting accounting.StoragenodeAccounting

	nowFn         func() time.Time
	lastTallyTime time.Time
	Node          map[storj.NodeID]float64
}

// NewRangedLoopObserver creates new RangedLoopObserver.
func NewRangedLoopObserver(log *zap.Logger, accounting accounting.StoragenodeAccounting) *RangedLoopObserver {
	return &RangedLoopObserver{
		log:        log,
		accounting: accounting,
		nowFn:      time.Now,
		Node:       map[storj.NodeID]float64{},
	}
}

// Start implements ranged loop observer start method.
func (observer *RangedLoopObserver) Start(ctx context.Context, time time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	observer.Node = map[storj.NodeID]float64{}
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
func (observer *RangedLoopObserver) Fork(ctx context.Context) (_ rangedloop.Partial, err error) {
	defer mon.Task()(&ctx)(&err)

	return NewRangedLoopPartial(observer.log, observer.nowFn), nil
}

// Join joins node tally ranged loop partial to main observer updating main per node usage map.
func (observer *RangedLoopObserver) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer mon.Task()(&ctx)(&err)

	tallyPartial, ok := partial.(*RangedLoopPartial)
	if !ok {
		return Error.New("expected partial type %T but got %T", tallyPartial, partial)
	}

	for nodeID, val := range tallyPartial.Node {
		observer.Node[nodeID] += val
	}

	return nil
}

// for backwards compatibility.
var monRangedTally = monkit.ScopeNamed("storj.io/storj/satellite/accounting/tally")

// Finish calculates byte*hours from per node storage usage and save tallies to DB.
func (observer *RangedLoopObserver) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	finishTime := observer.nowFn()

	// calculate byte hours, not just bytes
	hours := finishTime.Sub(observer.lastTallyTime).Hours()
	byteHours := make(map[storj.NodeID]float64)
	var totalSum float64
	for id, pieceSize := range observer.Node {
		totalSum += pieceSize
		byteHours[id] = pieceSize * hours
	}

	monRangedTally.IntVal("nodetallies.totalsum").Observe(int64(totalSum)) //mon:locked

	err = observer.accounting.SaveTallies(ctx, finishTime, byteHours)
	if err != nil {
		return Error.New("StorageNodeAccounting.SaveTallies failed: %v", err)
	}

	return nil
}

// SetNow overrides the timestamp used to store the result.
func (observer *RangedLoopObserver) SetNow(nowFn func() time.Time) {
	observer.nowFn = nowFn
}

// RangedLoopPartial implements node tally ranged loop partial.
type RangedLoopPartial struct {
	log   *zap.Logger
	nowFn func() time.Time

	Node map[storj.NodeID]float64
}

// NewRangedLoopPartial creates new node tally ranged loop partial.
func NewRangedLoopPartial(log *zap.Logger, nowFn func() time.Time) *RangedLoopPartial {
	return &RangedLoopPartial{
		log:   log,
		nowFn: nowFn,
		Node:  map[storj.NodeID]float64{},
	}
}

// Process iterates over segment range updating partial node usage map.
func (partial *RangedLoopPartial) Process(ctx context.Context, segments []segmentloop.Segment) error {
	now := partial.nowFn()

	for _, segment := range segments {
		partial.processSegment(now, segment)
	}

	return nil
}

func (partial *RangedLoopPartial) processSegment(now time.Time, segment segmentloop.Segment) {
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

	pieceSize := float64(segment.EncryptedSize / int32(minimumRequired)) // TODO: Add this as a method to RedundancyScheme

	for _, piece := range segment.Pieces {
		partial.Node[piece.StorageNode] += pieceSize
	}
}
