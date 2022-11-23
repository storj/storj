// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
)

var (
	// ContainError is the containment errs class.
	ContainError = errs.Class("containment")

	// ErrContainedNotFound is the errs class for when a pending audit isn't found.
	ErrContainedNotFound = errs.Class("pending audit not found")

	// ErrContainDelete is the errs class for when a pending audit can't be deleted.
	ErrContainDelete = errs.Class("unable to delete pending audit")
)

// NewContainment holds information about pending audits for contained nodes.
//
// It will exist side by side with Containment for a few commits in this
// commit chain, to allow the change in reverifications to be made over
// several smaller commits.
//
// Later in the commit chain, NewContainment will be renamed to Containment.
//
// architecture: Database
type NewContainment interface {
	Get(ctx context.Context, nodeID pb.NodeID) (*ReverificationJob, error)
	Insert(ctx context.Context, job *PieceLocator) error
	Delete(ctx context.Context, job *PieceLocator) (wasDeleted, nodeStillContained bool, err error)
}
