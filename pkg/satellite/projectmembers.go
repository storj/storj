// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// ProjectMembers exposes methods to manage ProjectMembers table in database.
type ProjectMembers interface {
	// GetByMemberID is a method for querying project members from the database by memberID.
	GetByMemberID(ctx context.Context, memberID uuid.UUID) ([]ProjectMember, error)
	// GetByProjectID is a method for querying project members from the database by projectID, offset and limit.
	GetByProjectID(ctx context.Context, projectID uuid.UUID, limit int, offset int64) ([]ProjectMember, error)
	// Insert is a method for inserting project member into the database.
	Insert(ctx context.Context, memberID, projectID uuid.UUID) (*ProjectMember, error)
	// Delete is a method for deleting project member by memberID and projectID from the database.
	Delete(ctx context.Context, memberID, projectID uuid.UUID) error
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
