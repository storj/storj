// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changehistory

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// ItemType represents the type of item being audited.
type ItemType string

const (
	// ItemTypeUser represents a user item.
	ItemTypeUser ItemType = "User"
	// ItemTypeProject represents a project item.
	ItemTypeProject ItemType = "Project"
	// ItemTypeBucket represents a bucket item.
	ItemTypeBucket ItemType = "Bucket"
)

// ChangeLog represents a log entry for a change made to an item.
type ChangeLog struct {
	UserID     uuid.UUID
	ProjectID  *uuid.UUID
	BucketName *string
	AdminEmail string
	ItemType   ItemType
	Reason     string
	Operation  string
	Changes    map[string]any
	Timestamp  time.Time
}

// DB defines the interface for logging and retrieving change history.
type DB interface {
	// LogChange logs a change to the change history.
	// the created ChangeLog is returned mostly for testing purposes.
	LogChange(ctx context.Context, params ChangeLog) (*ChangeLog, error)
	// TestListChangesByUserID lists change logs for a given user ID, ordered by timestamp descending.
	// This method is intended for testing purposes only.
	TestListChangesByUserID(ctx context.Context, userID uuid.UUID) ([]*ChangeLog, error)
}
