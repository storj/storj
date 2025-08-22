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

// Containment holds information about pending audits for contained nodes.
//
// architecture: Database
type Containment interface {
	Get(ctx context.Context, nodeID pb.NodeID) (*ReverificationJob, error)
	Insert(ctx context.Context, job *PieceLocator) error
	Delete(ctx context.Context, job *PieceLocator) (wasDeleted, nodeStillContained bool, err error)
	GetAllContainedNodes(ctx context.Context) ([]pb.NodeID, error)
}

// WrappedContainment is a typed Containment, helping mud to manage implementations.
type WrappedContainment struct {
	Containment
}

// NoContainment is a Containment implementation that does nothing.
type NoContainment struct {
}

// Get implements Containment.
func (n NoContainment) Get(ctx context.Context, nodeID pb.NodeID) (*ReverificationJob, error) {
	return nil, nil
}

// Insert implements Containment.
func (n NoContainment) Insert(ctx context.Context, job *PieceLocator) error {
	return nil
}

// Delete implements Containment.
func (n NoContainment) Delete(ctx context.Context, job *PieceLocator) (wasDeleted, nodeStillContained bool, err error) {
	return false, false, nil
}

// GetAllContainedNodes implements Containment.
func (n NoContainment) GetAllContainedNodes(ctx context.Context) ([]pb.NodeID, error) {
	return []pb.NodeID{}, nil
}

var _ Containment = &NoContainment{}
