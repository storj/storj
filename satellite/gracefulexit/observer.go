// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/segmentloop"
	"storj.io/storj/satellite/overlay"
)

// Observer populates the transfer queue for exiting nodes. It also updates the
// timed out status and removes transefer queue items for inactive exiting
// nodes.
type Observer struct {
	log     *zap.Logger
	db      DB
	overlay overlay.DB
	config  Config

	// The following variables are reset on each loop cycle
	exitingNodes    storj.NodeIDList
	bytesToTransfer map[storj.NodeID]int64
}

var _ rangedloop.Observer = (*Observer)(nil)
var _ rangedloop.Partial = (*observerFork)(nil)

// NewObserver returns a new ranged loop observer.
func NewObserver(log *zap.Logger, db DB, overlay overlay.DB, config Config) *Observer {
	return &Observer{
		log:     log,
		db:      db,
		overlay: overlay,
		config:  config,
	}
}

// Start updates the status and clears the transfer queue for inactive exiting
// nodes. It then prepares to populate the transfer queue for newly exiting
// nodes during the ranged loop cycle.
func (obs *Observer) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Determine which exiting nodes have yet to have complete a segment loop
	// that queues up related pieces for transfer.
	exitingNodes, err := obs.overlay.GetExitingNodes(ctx)
	if err != nil {
		return err
	}

	nodeCount := len(exitingNodes)
	if nodeCount == 0 {
		return nil
	}

	obs.log.Debug("found exiting nodes", zap.Int("exitingNodes", nodeCount))

	obs.checkForInactiveNodes(ctx, exitingNodes)

	obs.exitingNodes = nil
	obs.bytesToTransfer = make(map[storj.NodeID]int64)
	for _, node := range exitingNodes {
		if node.ExitLoopCompletedAt == nil {
			obs.exitingNodes = append(obs.exitingNodes, node.NodeID)
		}
	}
	return nil
}

// Fork returns path collector that will populate the transfer queue for
// segments belonging to newly exiting nodes for its range.
func (obs *Observer) Fork(ctx context.Context) (_ rangedloop.Partial, err error) {
	defer mon.Task()(&ctx)(&err)

	return newObserverFork(obs.log, obs.db, obs.exitingNodes, obs.config.ChoreBatchSize), nil
}

// Join flushes the forked path collector and aggregates collected metrics.
func (obs *Observer) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer mon.Task()(&ctx)(&err)

	pathCollector, ok := partial.(*observerFork)
	if !ok {
		return Error.New("expected partial type %T but got %T", pathCollector, partial)
	}

	if err := pathCollector.flush(ctx, 1); err != nil {
		return err
	}

	for nodeID, bytesToTransfer := range pathCollector.nodeIDStorage {
		obs.bytesToTransfer[nodeID] += bytesToTransfer
	}
	return nil
}

// Finish marks that the exit loop has been completed for newly exiting nodes
// that were processed in this loop cycle.
func (obs *Observer) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Record that the exit loop was completed for each node
	now := time.Now().UTC()
	for nodeID, bytesToTransfer := range obs.bytesToTransfer {
		exitStatus := overlay.ExitStatusRequest{
			NodeID:              nodeID,
			ExitLoopCompletedAt: now,
		}
		if _, err := obs.overlay.UpdateExitStatus(ctx, &exitStatus); err != nil {
			obs.log.Error("error updating exit status.", zap.Error(err))
		}
		mon.IntVal("graceful_exit_init_bytes_stored").Observe(bytesToTransfer)
	}
	return nil
}

func (obs *Observer) checkForInactiveNodes(ctx context.Context, exitingNodes []*overlay.ExitStatus) {
	for _, node := range exitingNodes {
		if node.ExitLoopCompletedAt == nil {
			// Node has not yet had all of its pieces added to the transfer queue
			continue
		}

		progress, err := obs.db.GetProgress(ctx, node.NodeID)
		if err != nil && !errs.Is(err, sql.ErrNoRows) {
			obs.log.Error("error retrieving progress for node", zap.Stringer("Node ID", node.NodeID), zap.Error(err))
			continue
		}

		lastActivityTime := *node.ExitLoopCompletedAt
		if progress != nil {
			lastActivityTime = progress.UpdatedAt
		}

		// check inactive timeframe
		if lastActivityTime.Add(obs.config.MaxInactiveTimeFrame).Before(time.Now().UTC()) {
			exitStatusRequest := &overlay.ExitStatusRequest{
				NodeID:         node.NodeID,
				ExitSuccess:    false,
				ExitFinishedAt: time.Now().UTC(),
			}
			mon.Meter("graceful_exit_fail_inactive").Mark(1)
			_, err = obs.overlay.UpdateExitStatus(ctx, exitStatusRequest)
			if err != nil {
				obs.log.Error("error updating exit status", zap.Error(err))
				continue
			}

			// remove all items from the transfer queue
			err := obs.db.DeleteTransferQueueItems(ctx, node.NodeID)
			if err != nil {
				obs.log.Error("error deleting node from transfer queue", zap.Error(err))
			}
		}
	}

}

type observerFork struct {
	log           *zap.Logger
	db            DB
	buffer        []TransferQueueItem
	batchSize     int
	nodeIDStorage map[storj.NodeID]int64
}

func newObserverFork(log *zap.Logger, db DB, exitingNodes storj.NodeIDList, batchSize int) *observerFork {
	fork := &observerFork{
		log:           log,
		db:            db,
		buffer:        make([]TransferQueueItem, 0, batchSize),
		batchSize:     batchSize,
		nodeIDStorage: make(map[storj.NodeID]int64, len(exitingNodes)),
	}

	if len(exitingNodes) > 0 {
		for _, nodeID := range exitingNodes {
			fork.nodeIDStorage[nodeID] = 0
		}
	}

	return fork
}

// Process adds transfer queue items for remote segments belonging to newly exiting nodes.
func (observer *observerFork) Process(ctx context.Context, segments []segmentloop.Segment) (err error) {
	// Intentionally omitting mon.Task here. The duration for all process
	// calls are aggregated and and emitted by the ranged loop service.

	if len(observer.nodeIDStorage) == 0 {
		return nil
	}

	for _, segment := range segments {
		if segment.Inline() {
			continue
		}
		if err := observer.handleRemoteSegment(ctx, segment); err != nil {
			return err
		}
	}
	return nil
}

func (observer *observerFork) handleRemoteSegment(ctx context.Context, segment segmentloop.Segment) (err error) {
	numPieces := len(segment.Pieces)
	for _, piece := range segment.Pieces {
		if _, ok := observer.nodeIDStorage[piece.StorageNode]; !ok {
			continue
		}

		pieceSize := segment.PieceSize()

		observer.nodeIDStorage[piece.StorageNode] += pieceSize

		item := TransferQueueItem{
			NodeID:          piece.StorageNode,
			StreamID:        segment.StreamID,
			Position:        segment.Position,
			PieceNum:        int32(piece.Number),
			RootPieceID:     segment.RootPieceID,
			DurabilityRatio: float64(numPieces) / float64(segment.Redundancy.TotalShares),
		}

		observer.log.Debug("adding piece to transfer queue.", zap.Stringer("Node ID", piece.StorageNode),
			zap.String("stream_id", segment.StreamID.String()), zap.Int32("part", int32(segment.Position.Part)),
			zap.Int32("index", int32(segment.Position.Index)), zap.Uint16("piece num", piece.Number),
			zap.Int("num pieces", numPieces), zap.Int16("total possible pieces", segment.Redundancy.TotalShares))

		observer.buffer = append(observer.buffer, item)
		err = observer.flush(ctx, observer.batchSize)
		if err != nil {
			return err
		}
	}

	return nil
}

func (observer *observerFork) flush(ctx context.Context, limit int) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(observer.buffer) >= limit {
		err = observer.db.Enqueue(ctx, observer.buffer, observer.batchSize)
		observer.buffer = observer.buffer[:0]

		return errs.Wrap(err)
	}
	return nil
}
