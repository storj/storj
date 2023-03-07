// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/overlay"
)

// ContainmentSyncChore is a chore to update the set of contained nodes in the
// overlay cache. This is necessary because it is possible for the "contained"
// field in the nodes table to disagree with whether a node appears in the
// reverification queue. We make an effort to keep them in sync when making
// changes to the reverification queue, but this infrequent chore will clean up
// any inconsistencies that creep in (because we can't maintain perfect
// consistency while the reverification queue and the nodes table may be in
// separate databases). Fortunately, it is acceptable for a node's containment
// status to be out of date for some amount of time.
type ContainmentSyncChore struct {
	log     *zap.Logger
	queue   ReverifyQueue
	overlay overlay.DB

	Loop *sync2.Cycle
}

// NewContainmentSyncChore creates a new ContainmentSyncChore.
func NewContainmentSyncChore(log *zap.Logger, queue ReverifyQueue, overlay overlay.DB, interval time.Duration) *ContainmentSyncChore {
	return &ContainmentSyncChore{
		log:     log,
		queue:   queue,
		overlay: overlay,
		Loop:    sync2.NewCycle(interval),
	}
}

// Run runs the reverify chore.
func (rc *ContainmentSyncChore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return rc.Loop.Run(ctx, rc.syncContainedStatus)
}

// SyncContainedStatus updates the contained status of all nodes in the overlay cache
// as necessary to match whether they currently appear in the reverification queue at
// least once.
func (rc *ContainmentSyncChore) syncContainedStatus(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	containedSet, err := rc.queue.GetAllContainedNodes(ctx)
	if err != nil {
		rc.log.Error("failed to get set of contained nodes from reverify queue", zap.Error(err))
		return nil
	}
	err = rc.overlay.SetAllContainedNodes(ctx, containedSet)
	if err != nil {
		rc.log.Error("failed to update the set of contained nodes in the overlay cache", zap.Error(err))
		return nil
	}
	rc.log.Info("updated containment status of all nodes as necessary",
		zap.Int("num contained nodes", len(containedSet)))
	return nil
}
