// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// ProjectMembers exposes methods to manage ProjectMembers table in database.
// TODO: some methods will be removed, some - added
type ProjectMembers interface {
	// GetAll is a method for querying all project members from the database.
	GetAll(ctx context.Context) ([]ProjectMember, error)
	// GetByMemberID is a method for querying project member from the database by memberID.
	GetByMemberID(ctx context.Context, memberID uuid.UUID) (*ProjectMember, error)
	// GetByProjectID is a method for querying project members from the database by projectID.
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]ProjectMember, error)
	// Get is a method for querying project member from the database by id.
	Get(ctx context.Context, id uuid.UUID) (*ProjectMember, error)
	// Insert is a method for inserting project member into the database.
	Insert(ctx context.Context, memberID, projectID uuid.UUID) (*ProjectMember, error)
	// Delete is a method for deleting project member by Id from the database.
	Delete(ctx context.Context, id uuid.UUID) error
	// Update is a method for updating project member entity.
	Update(ctx context.Context, projectMember *ProjectMember) error
}

// ProjectMember is a database object that describes ProjectMember entity.
type ProjectMember struct {
	ID uuid.UUID

	// FK on Users table.
	MemberID uuid.UUID
	// FK on Projects table.
	ProjectID uuid.UUID

	CreatedAt time.Time
}
