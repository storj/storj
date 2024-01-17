// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

import (
	"context"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// DB implements the database for node events.
//
// architecture: Database
type DB interface {
	// Insert a node event into the node events table.
	Insert(ctx context.Context, email string, lastIPPort *string, nodeID storj.NodeID, event Type) (nodeEvent NodeEvent, err error)
	// GetLatestByEmailAndEvent gets latest node event by email and event type.
	GetLatestByEmailAndEvent(ctx context.Context, email string, event Type) (nodeEvent NodeEvent, err error)
	// GetNextBatch gets the next batch of events to combine into an email.
	GetNextBatch(ctx context.Context, firstSeenBefore time.Time) (events []NodeEvent, err error)
	// GetByID get a node event by id.
	GetByID(ctx context.Context, id uuid.UUID) (nodeEvent NodeEvent, err error)
	// UpdateEmailSent updates email_sent for a group of rows.
	UpdateEmailSent(ctx context.Context, ids []uuid.UUID, timestamp time.Time) (err error)
	// UpdateLastAttempted updates last_attempted for a group of rows.
	UpdateLastAttempted(ctx context.Context, ids []uuid.UUID, timestamp time.Time) (err error)
}

// NodeEvent contains information needed to notify a node operator about something that happened to a node.
type NodeEvent struct {
	ID            uuid.UUID
	Email         string
	LastIPPort    *string
	NodeID        storj.NodeID
	Event         Type
	CreatedAt     time.Time
	LastAttempted *time.Time
	EmailSent     *time.Time
}
