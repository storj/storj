// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// ProjectInvitations exposes methods to manage pending project member invitations in the database.
//
// architecture: Database
type ProjectInvitations interface {
	// Upsert updates a project member invitation if it exists and inserts it otherwise.
	Upsert(ctx context.Context, invite *ProjectInvitation) (*ProjectInvitation, error)
	// Get returns a project member invitation from the database.
	Get(ctx context.Context, projectID uuid.UUID, email string) (*ProjectInvitation, error)
	// GetByProjectID returns all the project member invitations for the project specified by the given ID.
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]ProjectInvitation, error)
	// GetByEmail returns all the project member invitations for the specified email address.
	GetByEmail(ctx context.Context, email string) ([]ProjectInvitation, error)
	// GetForActiveProjectsByEmail returns all project member invitations associated with active projects for the specified email address.
	GetForActiveProjectsByEmail(ctx context.Context, email string) ([]ProjectInvitation, error)
	// Delete removes a project member invitation from the database.
	Delete(ctx context.Context, projectID uuid.UUID, email string) error
}

// ProjectInvitation represents a pending project member invitation.
type ProjectInvitation struct {
	ProjectID uuid.UUID
	Email     string
	InviterID *uuid.UUID
	CreatedAt time.Time
}
