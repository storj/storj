// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package bucketmigrations

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

var (
	// Error is the default error class for bucket migrations.
	Error = errs.Class("bucket migrations")

	// ErrMigrationNotFound is used when a migration is not found.
	ErrMigrationNotFound = errs.Class("migration not found")
)

// MigrationType represents the type of migration.
type MigrationType int

const (
	// MigrationTypeTrivial changes placement ID on segments and bucket metadata when erasure coding parameters match.
	MigrationTypeTrivial MigrationType = 0
	// MigrationTypeFull re-encodes and redistributes data when moving to placements with different RS parameters.
	MigrationTypeFull MigrationType = 1
)

// State represents the current state of a migration.
type State string

const (
	// StatePending indicates the migration has been created but not yet started.
	StatePending State = "pending"
	// StateInProgress indicates the migration is currently being processed.
	StateInProgress State = "in_progress"
	// StateCompleted indicates the migration has completed successfully.
	StateCompleted State = "completed"
	// StateFailed indicates the migration has failed.
	StateFailed State = "failed"
	// StateCancelled indicates the migration was cancelled.
	StateCancelled State = "cancelled"
)

// Migration represents a bucket migration job.
type Migration struct {
	ID             uuid.UUID
	ProjectID      uuid.UUID
	BucketName     string
	FromPlacement  int
	ToPlacement    int
	MigrationType  MigrationType
	State          State
	BytesProcessed uint64
	ErrorMessage   *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

// UpdateFields contains the fields that can be updated for a migration.
type UpdateFields struct {
	State          *State
	BytesProcessed *uint64
	ErrorMessage   *string
	CompletedAt    *time.Time
}

// ListOptions contains options for listing migrations.
type ListOptions struct {
	State  State
	Limit  int
	Offset int
}

// DB is the interface for the database to interact with bucket migrations.
//
// architecture: Database
type DB interface {
	// Create creates a new migration record.
	Create(ctx context.Context, migration Migration) (_ Migration, err error)
	// Get retrieves a migration by ID.
	Get(ctx context.Context, id uuid.UUID) (_ Migration, err error)
	// Update updates a migration's fields.
	Update(ctx context.Context, id uuid.UUID, update UpdateFields) (err error)
	// Delete removes a migration record.
	Delete(ctx context.Context, id uuid.UUID) (err error)
	// ListByBucket returns all migrations for a specific bucket, ordered by created_at descending.
	ListByBucket(ctx context.Context, projectID uuid.UUID, bucketName string) (migrations []Migration, err error)
	// ListByState returns migrations in a specific state with pagination.
	ListByState(ctx context.Context, opts ListOptions) (migrations []Migration, err error)
}
