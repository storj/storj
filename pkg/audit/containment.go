// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var (
	// ContainError is the containment errs class
	ContainError = errs.Class("containment error")

	// ErrContainedNotFound is the errs class for when a pending audit isn't found
	ErrContainedNotFound = errs.Class("pending audit not found")

	// ErrContainDelete is the errs class for when a pending audit can't be deleted
	ErrContainDelete = errs.Class("unable to delete pending audit")
)

// PendingAudit contains info needed for retrying an audit for a contained node
type PendingAudit struct {
	NodeID            storj.NodeID
	PieceID           storj.PieceID
	StripeIndex       int
	ShareSize         int64
	ExpectedShareHash []byte
	ReverifyCount     int
}

// DB holds information about pending audits for contained nodes
type DB interface {
	Get(ctx context.Context, nodeID pb.NodeID) (*PendingAudit, error)
	IncrementPending(ctx context.Context, pendingAudit *PendingAudit) error
	Delete(ctx context.Context, nodeID pb.NodeID) error
}

// Containment allows access to pending audit info in the db
type Containment struct {
	log *zap.Logger
	db  DB
}

// NewContainment creates a new containment db
func NewContainment(log *zap.Logger, db DB) *Containment {
	return &Containment{
		log: log,
		db:  db,
	}
}

// Get gets pending audit info from the db
func (containment *Containment) Get(ctx context.Context, nodeID pb.NodeID) (*PendingAudit, error) {
	pendingAudit, err := containment.db.Get(ctx, nodeID)
	if err != nil {
		return nil, ContainError.Wrap(err)
	}
	return pendingAudit, nil
}

// IncrementPending either creates a new entry for a pending audit or updates an existing pending audit's reverify count
func (containment *Containment) IncrementPending(ctx context.Context, pending *PendingAudit) error {
	err := containment.db.IncrementPending(ctx, pending)
	if err != nil {
		return ContainError.Wrap(err)
	}
	return nil
}

// Delete deletes pending audit info from the db
func (containment *Containment) Delete(ctx context.Context, nodeID pb.NodeID) error {
	err := containment.db.Delete(ctx, nodeID)
	if err != nil {
		return ContainError.Wrap(err)
	}
	return nil
}
