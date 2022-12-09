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

	// TODO: trim out/refactor segmentloop.Observer bits from path collector
	// once segmentloop.Observer is removed.
	return NewPathCollector(obs.log, obs.db, obs.exitingNodes, obs.config.ChoreBatchSize), nil
}

// Join flushes the forked path collector and aggregates collected metrics.
func (obs *Observer) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer mon.Task()(&ctx)(&err)

	pathCollector, ok := partial.(*PathCollector)
	if !ok {
		return Error.New("expected partial type %T but got %T", pathCollector, partial)
	}

	if err := pathCollector.Flush(ctx); err != nil {
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
