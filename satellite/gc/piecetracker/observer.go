// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package piecetracker

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/overlay"
)

var (
	// Error is a standard error class for this package.
	Error = errs.Class("piecetracker")
	mon   = monkit.Package()

	// check if Observer and Partial interfaces are satisfied.
	_ rangedloop.Observer = (*Observer)(nil)
	_ rangedloop.Partial  = (*observerFork)(nil)
)

// Observer implements piecetraker ranged loop observer.
//
// The piecetracker counts the number of pieces currently expected to reside on each node,
// then passes the counts to the overlay with UpdatePieceCounts().
type Observer struct {
	log        *zap.Logger
	config     Config
	overlay    overlay.DB
	metabaseDB *metabase.DB

	pieceCounts map[metabase.NodeAlias]int64
}

// NewObserver creates new piecetracker ranged loop observer.
func NewObserver(log *zap.Logger, metabaseDB *metabase.DB, overlay overlay.DB, config Config) *Observer {
	return &Observer{
		log:         log,
		overlay:     overlay,
		metabaseDB:  metabaseDB,
		config:      config,
		pieceCounts: map[metabase.NodeAlias]int64{},
	}
}

// Start implements ranged loop observer start method.
func (observer *Observer) Start(ctx context.Context, time time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	observer.pieceCounts = map[metabase.NodeAlias]int64{}
	return nil
}

// Fork implements ranged loop observer fork method.
func (observer *Observer) Fork(ctx context.Context) (_ rangedloop.Partial, err error) {
	defer mon.Task()(&ctx)(&err)

	return newObserverFork(), nil
}

// Join joins piecetracker ranged loop partial to main observer updating piece counts map.
func (observer *Observer) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer mon.Task()(&ctx)(&err)
	pieceTracker, ok := partial.(*observerFork)
	if !ok {
		return Error.New("expected %T but got %T", pieceTracker, partial)
	}

	// Merge piece counts for each node.
	for nodeAlias, pieceCount := range pieceTracker.pieceCounts {
		observer.pieceCounts[nodeAlias] += pieceCount
	}

	return nil
}

// Finish updates piece counts in the DB.
func (observer *Observer) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	observer.log.Info("piecetracker observer finished")

	nodeAliasMap, err := observer.metabaseDB.LatestNodesAliasMap(ctx)
	nodesToUpdate := make(map[storj.NodeID]int64, observer.config.UpdateBatchSize)

	updateNodes := func(nodesToUpdate map[storj.NodeID]int64) {
		err = observer.overlay.UpdatePieceCounts(ctx, nodesToUpdate)
		if err != nil {
			// don't stop on error as updating always all nodes is not critical
			// missed numbers will be updated with next iterations
			observer.log.Error("error updating nodes piece counts", zap.Error(err))
		}
	}

	for nodeAlias, count := range observer.pieceCounts {
		nodeID, ok := nodeAliasMap.Node(nodeAlias)
		if !ok {
			observer.log.Error("unrecognized node alias in piecetracker ranged-loop", zap.Int32("node-alias", int32(nodeAlias)))
			continue
		}
		nodesToUpdate[nodeID] = count

		if len(nodesToUpdate) >= observer.config.UpdateBatchSize {
			updateNodes(nodesToUpdate)

			maps.Clear(nodesToUpdate)
		}
	}

	updateNodes(nodesToUpdate)

	return nil
}

type observerFork struct {
	pieceCounts map[metabase.NodeAlias]int64
}

// newObserverFork creates new piecetracker ranged loop fork.
func newObserverFork() *observerFork {
	return &observerFork{
		pieceCounts: map[metabase.NodeAlias]int64{},
	}
}

// Process iterates over segment range updating partial piece counts for each node.
func (fork *observerFork) Process(ctx context.Context, segments []rangedloop.Segment) error {
	now := time.Now()
	for _, segment := range segments {
		if segment.Inline() || segment.Expired(now) {
			continue
		}

		for _, piece := range segment.AliasPieces {
			fork.pieceCounts[piece.Alias]++
		}
	}

	return nil
}
