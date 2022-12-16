// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

var (
	// ContainError is the containment errs class.
	ContainError = errs.Class("containment")

	// ErrContainedNotFound is the errs class for when a pending audit isn't found.
	ErrContainedNotFound = errs.Class("pending audit not found")

	// ErrContainDelete is the errs class for when a pending audit can't be deleted.
	ErrContainDelete = errs.Class("unable to delete pending audit")
)

// PendingAudit contains info needed for retrying an audit for a contained node.
type PendingAudit struct {
	NodeID            storj.NodeID
	PieceID           storj.PieceID
	StripeIndex       int32
	ShareSize         int32
	ExpectedShareHash []byte
	ReverifyCount     int32
	StreamID          uuid.UUID
	Position          metabase.SegmentPosition
}

// Containment holds information about pending audits for contained nodes.
//
// architecture: Database
type Containment interface {
	Get(ctx context.Context, nodeID pb.NodeID) (*PendingAudit, error)
	IncrementPending(ctx context.Context, pendingAudit *PendingAudit) error
	Delete(ctx context.Context, nodeID pb.NodeID) (bool, error)
}

// NewContainment holds information about pending audits for contained nodes.
//
// It will exist side by side with Containment for a few commits in this
// commit chain, to allow the change in reverifications to be made over
// several smaller commits.
//
// Later in the commit chain, NewContainment will replace Containment.
type NewContainment interface {
	Get(ctx context.Context, nodeID pb.NodeID) (*ReverificationJob, error)
	Insert(ctx context.Context, job *PieceLocator) error
	Delete(ctx context.Context, job *PieceLocator) (wasDeleted, nodeStillContained bool, err error)
}
