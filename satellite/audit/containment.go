// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metainfo/metabase"
)

var (
	// ContainError is the containment errs class.
	ContainError = errs.Class("containment error")

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
	Segment           metabase.SegmentLocation
}

// Containment holds information about pending audits for contained nodes.
//
// architecture: Database
type Containment interface {
	Get(ctx context.Context, nodeID pb.NodeID) (*PendingAudit, error)
	IncrementPending(ctx context.Context, pendingAudit *PendingAudit) error
	Delete(ctx context.Context, nodeID pb.NodeID) (bool, error)
}
