// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package planneddowntime

import (
	"context"
	"time"
)

// DB works with planned downtime database.
//
// architecture: Database
type DB interface {
	// Add inserts piece information into the database.
	Add(ctx context.Context, planned Entry) error
	// GetScheduled gets a list of current and future planned downtimes.
	GetScheduled(ctx context.Context, since time.Time) ([]Entry, error)
	// GetCompleted gets a list of completed planned downtimes.
	GetCompleted(ctx context.Context, since time.Time) ([]Entry, error)
	// Delete deletes an existing planned downtime entry.
	Delete(ctx context.Context, id []byte) error
}

// Entry defines a single planned downtime entry.
type Entry struct {
	ID                      []byte
	Start, End, ScheduledAt time.Time
}
