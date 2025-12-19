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
	ID         uuid.UUID      `json:"id"`
	UserID     uuid.UUID      `json:"userID"`
	ProjectID  *uuid.UUID     `json:"projectID,omitempty"`
	BucketName *string        `json:"bucketName,omitempty"`
	AdminEmail string         `json:"adminEmail"`
	ItemType   ItemType       `json:"itemType"`
	Reason     string         `json:"reason"`
	Operation  string         `json:"operation"`
	Changes    map[string]any `json:"changes"`
	Timestamp  time.Time      `json:"timestamp"`
}

// DB defines the interface for logging and retrieving change history.
type DB interface {
	// LogChange logs a change to the change history.
	// the created ChangeLog is returned mostly for testing purposes.
	LogChange(ctx context.Context, params ChangeLog) (*ChangeLog, error)
	// GetChangesByUserID retrieves the change history for a specific user.
	// If exact is false, changes to the user's projects and buckets are also included.
	GetChangesByUserID(ctx context.Context, userID uuid.UUID, exact bool) (_ []ChangeLog, err error)
	// GetChangesByProjectID retrieves the change history for a specific project.
	// If exact is false, changes to the project's buckets are also included.
	GetChangesByProjectID(ctx context.Context, projectId uuid.UUID, exact bool) (_ []ChangeLog, err error)
	// GetChangesByBucketName retrieves the change history for a specific bucket.
	GetChangesByBucketName(ctx context.Context, bucketName string) (_ []ChangeLog, err error)
}
