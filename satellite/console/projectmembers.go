// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

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
	GetByProjectID(ctx context.Context, projectID uuid.UUID, pagination Pagination) ([]ProjectMember, error)
	// Insert is a method for inserting project member into the database.
	Insert(ctx context.Context, memberID, projectID uuid.UUID) (*ProjectMember, error)
	// Delete is a method for deleting project member by memberID and projectID from the database.
	Delete(ctx context.Context, memberID, projectID uuid.UUID) error
}

// ProjectMember is a database object that describes ProjectMember entity.
type ProjectMember struct {
	// FK on Users table.
	MemberID uuid.UUID
	// FK on Projects table.
	ProjectID uuid.UUID

	CreatedAt time.Time
}

// Pagination defines pagination, filtering and sorting rules
type Pagination struct {
	Limit  int
	Offset int64
	Search string
	Order  ProjectMemberOrder
}

// ProjectMemberOrder is used for querying project members in specified order
type ProjectMemberOrder int8

const (
	// Name indicates that we should order by full name
	Name ProjectMemberOrder = 1
	// Email indicates that we should order by email
	Email ProjectMemberOrder = 2
	// Created indicates that we should order by created date
	Created ProjectMemberOrder = 3
)
