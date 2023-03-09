// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/common/pb"
	"storj.io/storj/satellite/audit"
)

type containment struct {
	reverifyQueue audit.ReverifyQueue
}

var _ audit.Containment = &containment{}

// Get gets a pending reverification audit by node id. If there are
// multiple pending reverification audits, an arbitrary one is returned.
// If there are none, an error wrapped by audit.ErrContainedNotFound is
// returned.
func (containment *containment) Get(ctx context.Context, id pb.NodeID) (_ *audit.ReverificationJob, err error) {
	defer mon.Task()(&ctx)(&err)
	if id.IsZero() {
		return nil, audit.ContainError.New("node ID empty")
	}

	return containment.reverifyQueue.GetByNodeID(ctx, id)
}

// Insert creates a new pending audit entry.
func (containment *containment) Insert(ctx context.Context, pendingJob *audit.PieceLocator) (err error) {
	defer mon.Task()(&ctx)(&err)

	return containment.reverifyQueue.Insert(ctx, pendingJob)
}

// Delete removes a job from the reverification queue, whether because the job
// was successful or because the job is no longer necessary. The wasDeleted
// return value indicates whether the indicated job was actually deleted (if
// not, there was no such job in the queue).
func (containment *containment) Delete(ctx context.Context, pendingJob *audit.PieceLocator) (isDeleted, nodeStillContained bool, err error) {
	defer mon.Task()(&ctx)(&err)

	isDeleted, err = containment.reverifyQueue.Remove(ctx, pendingJob)
	if err != nil {
		return false, false, audit.ContainError.Wrap(err)
	}

	nodeStillContained = true
	_, err = containment.reverifyQueue.GetByNodeID(ctx, pendingJob.NodeID)
	if audit.ErrContainedNotFound.Has(err) {
		nodeStillContained = false
		err = nil
	}
	return isDeleted, nodeStillContained, audit.ContainError.Wrap(err)
}

func (containment *containment) GetAllContainedNodes(ctx context.Context) (nodes []pb.NodeID, err error) {
	defer mon.Task()(&ctx)(&err)

	return containment.reverifyQueue.GetAllContainedNodes(ctx)
}
