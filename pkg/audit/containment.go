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

	// ErrAlreadyExists is the errs class for when a pending audit with the same nodeID but different share data already exists
	ErrAlreadyExists = errs.Class("pending audit already exists for nodeID")
)

// PendingAudit contains info needed for retrying an audit for a contained node
type PendingAudit struct {
	NodeID            storj.NodeID
	PieceID           storj.PieceID
	StripeIndex       uint32
	ShareSize         int64
	ExpectedShareHash []byte
	ReverifyCount     uint32
}

// DB holds information about pending audits for contained nodes
type DB interface {
	Get(ctx context.Context, nodeID pb.NodeID) (*PendingAudit, error)
	IncrementPending(ctx context.Context, pendingAudit *PendingAudit) error
	Delete(ctx context.Context, nodeID pb.NodeID) (bool, error)
}

// Containment allows the audit verifier to access pending audit info in the db (once it's added to the verifier struct)
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
	return pendingAudit, ContainError.Wrap(err)
}

// IncrementPending either creates a new entry for a pending audit or updates an existing pending audit's reverify count
func (containment *Containment) IncrementPending(ctx context.Context, pending *PendingAudit) error {
	err := containment.db.IncrementPending(ctx, pending)
	return ContainError.Wrap(err)
}

// Delete deletes pending audit info from the db
func (containment *Containment) Delete(ctx context.Context, nodeID pb.NodeID) (bool, error) {
	isDeleted, err := containment.db.Delete(ctx, nodeID)
	return isDeleted, ContainError.Wrap(err)
}
