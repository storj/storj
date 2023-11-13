// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// This whole file can be removed when we are using TimeBased GE everywhere.

// Progress represents the persisted graceful exit progress record.
type Progress struct {
	NodeID            storj.NodeID
	BytesTransferred  int64
	PiecesTransferred int64
	PiecesFailed      int64
	UpdatedAt         time.Time
}

// TransferQueueItem represents the persisted graceful exit queue record.
type TransferQueueItem struct {
	NodeID              storj.NodeID
	StreamID            uuid.UUID
	Position            metabase.SegmentPosition
	PieceNum            int32
	RootPieceID         storj.PieceID
	DurabilityRatio     float64
	QueuedAt            time.Time
	RequestedAt         *time.Time
	LastFailedAt        *time.Time
	LastFailedCode      *int
	FailedCount         *int
	FinishedAt          *time.Time
	OrderLimitSendCount int
}

// DB implements CRUD operations for graceful exit service.
//
// architecture: Database
type DB interface {
	// IncrementProgress increments transfer stats for a node.
	IncrementProgress(ctx context.Context, nodeID storj.NodeID, bytes int64, successfulTransfers int64, failedTransfers int64) error
	// GetProgress gets a graceful exit progress entry.
	GetProgress(ctx context.Context, nodeID storj.NodeID) (*Progress, error)

	// Enqueue batch inserts graceful exit transfer queue entries it does not exist.
	Enqueue(ctx context.Context, items []TransferQueueItem, batchSize int) error
	// UpdateTransferQueueItem creates a graceful exit transfer queue entry.
	UpdateTransferQueueItem(ctx context.Context, item TransferQueueItem) error
	// DeleteTransferQueueItem deletes a graceful exit transfer queue entry.
	DeleteTransferQueueItem(ctx context.Context, nodeID storj.NodeID, StreamID uuid.UUID, Position metabase.SegmentPosition, pieceNum int32) error
	// DeleteTransferQueueItem deletes a graceful exit transfer queue entries by nodeID.
	DeleteTransferQueueItems(ctx context.Context, nodeID storj.NodeID) error
	// DeleteFinishedTransferQueueItem deletes finished graceful exit transfer queue entries.
	DeleteFinishedTransferQueueItems(ctx context.Context, nodeID storj.NodeID) error
	// DeleteAllFinishedTransferQueueItems deletes all graceful exit transfer
	// queue items whose nodes have finished the exit before the indicated time
	// returning the total number of deleted items.
	DeleteAllFinishedTransferQueueItems(ctx context.Context, before time.Time, asOfSystemTimeInterval time.Duration, batchSize int) (count int64, err error)
	// DeleteFinishedExitProgress deletes exit progress entries for nodes that
	// finished exiting before the indicated time, returns number of deleted entries.
	DeleteFinishedExitProgress(ctx context.Context, before time.Time, asOfSystemTimeInterval time.Duration) (count int64, err error)
	// GetFinishedExitNodes gets nodes that are marked having finished graceful exit before a given time.
	GetFinishedExitNodes(ctx context.Context, before time.Time, asOfSystemTimeInterval time.Duration) (finishedNodes []storj.NodeID, err error)
	// GetTransferQueueItem gets a graceful exit transfer queue entry.
	GetTransferQueueItem(ctx context.Context, nodeID storj.NodeID, StreamID uuid.UUID, Position metabase.SegmentPosition, pieceNum int32) (*TransferQueueItem, error)
	// GetIncomplete gets incomplete graceful exit transfer queue entries ordered by durability ratio and queued date ascending.
	GetIncomplete(ctx context.Context, nodeID storj.NodeID, limit int, offset int64) ([]*TransferQueueItem, error)
	// GetIncompleteNotFailed gets incomplete graceful exit transfer queue entries in the database ordered by durability ratio and queued date ascending.
	GetIncompleteNotFailed(ctx context.Context, nodeID storj.NodeID, limit int, offset int64) ([]*TransferQueueItem, error)
	// GetIncompleteNotFailed gets incomplete graceful exit transfer queue entries that have failed <= maxFailures times, ordered by durability ratio and queued date ascending.
	GetIncompleteFailed(ctx context.Context, nodeID storj.NodeID, maxFailures int, limit int, offset int64) ([]*TransferQueueItem, error)
	// IncrementOrderLimitSendCount increments the number of times a node has been sent an order limit for transferring.
	IncrementOrderLimitSendCount(ctx context.Context, nodeID storj.NodeID, StreamID uuid.UUID, Position metabase.SegmentPosition, pieceNum int32) error
	// CountFinishedTransferQueueItemsByNode return a map of the nodes which has
	// finished the exit before the indicated time but there are at least one item
	// left in the transfer queue.
	CountFinishedTransferQueueItemsByNode(ctx context.Context, before time.Time, asOfSystemTimeInterval time.Duration) (map[storj.NodeID]int64, error)
}
