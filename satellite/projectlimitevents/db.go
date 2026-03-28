// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package projectlimitevents

import (
	"context"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
)

// DB implements the database for project limit events.
//
// architecture: Database
type DB interface {
	// Insert adds a new project limit event to the queue.
	Insert(ctx context.Context, projectID uuid.UUID, event accounting.ProjectUsageThreshold, isReset bool) (Event, error)
	// GetByID returns a single event by ID.
	GetByID(ctx context.Context, id uuid.UUID) (Event, error)
	// GetNextBatch returns all unprocessed events for the project that has the
	// oldest unprocessed event created before firstSeenBefore.
	GetNextBatch(ctx context.Context, firstSeenBefore time.Time) ([]Event, error)
	// UpdateEmailSent marks a group of events as processed.
	UpdateEmailSent(ctx context.Context, ids []uuid.UUID, timestamp time.Time) error
	// UpdateLastAttempted updates last_attempted for a group of events.
	UpdateLastAttempted(ctx context.Context, ids []uuid.UUID, timestamp time.Time) error
}

// Event contains a project limit threshold event from the queue.
type Event struct {
	ID            uuid.UUID
	ProjectID     uuid.UUID
	Event         accounting.ProjectUsageThreshold
	IsReset       bool
	CreatedAt     time.Time
	LastAttempted *time.Time
	EmailSent     *time.Time
}
