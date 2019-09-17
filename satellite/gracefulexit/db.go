// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"time"

	"storj.io/storj/pkg/storj"
)

// Progress represents the persisted graceful exit progress record
type Progress struct {
	NodeID           storj.NodeID
	BytesTransferred int64
	UpdatedAt        time.Time
}

// TransferQueueItem represents the persisted graceful exit queue record
type TransferQueueItem struct {
	NodeID          storj.NodeID
	Path            []byte
	PieceNum        int
	DurabilityRatio float64
	QueuedAt        time.Time
	RequestedAt     time.Time
	LastFailedAt    time.Time
	LastFailedCode  int
	FailedCount     int
	FinishedAt      time.Time
}

// DB implements CRUD operations for graceful exit service
//
// architecture: Database
type DB interface {
	// CreateProgress creates a graceful exit progress entry in the database.
	CreateProgress(ctx context.Context, nodeID storj.NodeID) error
	// UpdateProgress updates a graceful exit progress entry in the database.
	UpdateProgress(ctx context.Context, nodeID storj.NodeID, bytesTransferred int64) error
	// DeleteProgress deletes a graceful exit progress entry in the database.
	DeleteProgress(ctx context.Context, nodeID storj.NodeID) error
	// IncrementProgressBytesTransferred increments bytes transferred value.
	IncrementProgressBytesTransferred(ctx context.Context, nodeID storj.NodeID, bytesTransferred int64) error
	// GetProgress gets a graceful exit progress entry in the database.
	GetProgress(ctx context.Context, nodeID storj.NodeID) (*Progress, error)

	// CreateTransferQueueItem creates a graceful exit transfer queue entry in the database.
	CreateTransferQueueItem(ctx context.Context, item TransferQueueItem) error
	// UpdateTransferQueueItem creates a graceful exit transfer queue entry in the database.
	UpdateTransferQueueItem(ctx context.Context, item TransferQueueItem) error
	// DeleteTransferQueueItem deletes a graceful exit transfer queue entry in the database.
	DeleteTransferQueueItem(ctx context.Context, nodeID storj.NodeID, path []byte) error
	// GetTransferQueueItem gets a graceful exit transfer queue entry in the database.
	GetTransferQueueItem(ctx context.Context, nodeID storj.NodeID, path []byte) (*TransferQueueItem, error)
	// GetIncompleteTransferQueueItemsByNodeIDWithLimits gets incomplete graceful exit transfer queue entries in the database ordered by the queued date ascending.
	GetIncompleteTransferQueueItemsByNodeIDWithLimits(ctx context.Context, nodeID storj.NodeID, limit int, offset int64) ([]*TransferQueueItem, error)
}
