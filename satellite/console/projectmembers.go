// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// ProjectMembers exposes methods to manage ProjectMembers table in database.
//
// architecture: Database
type ProjectMembers interface {
	// GetByMemberID is a method for querying project members from the database by memberID.
	GetByMemberID(ctx context.Context, memberID uuid.UUID) ([]ProjectMember, error)
	// GetByMemberIDAndProjectID is a method for querying project member from the database by memberID and projectID.
	GetByMemberIDAndProjectID(ctx context.Context, memberID, projectID uuid.UUID) (*ProjectMember, error)
	// GetPagedWithInvitationsByProjectID is a method for querying project members and invitations from the database by projectID and cursor.
	GetPagedWithInvitationsByProjectID(ctx context.Context, projectID uuid.UUID, cursor ProjectMembersCursor) (*ProjectMembersPage, error)
	// GetTotalCountByProjectID is a method for getting total count of project members by projectID.
	GetTotalCountByProjectID(ctx context.Context, projectID uuid.UUID) (uint64, error)
	// UpdateRole is a method for updating project member role in the database.
	UpdateRole(ctx context.Context, memberID, projectID uuid.UUID, newRole ProjectMemberRole) (*ProjectMember, error)
	// Insert is a method for inserting project member into the database.
	Insert(ctx context.Context, memberID, projectID uuid.UUID, role ProjectMemberRole) (*ProjectMember, error)
	// Delete is a method for deleting project member by memberID and projectID from the database.
	Delete(ctx context.Context, memberID, projectID uuid.UUID) error
}

// ProjectMember is a database object that describes ProjectMember entity.
type ProjectMember struct {
	// FK on Users table.
	MemberID uuid.UUID
	// FK on Projects table.
	ProjectID uuid.UUID

	Role ProjectMemberRole

	CreatedAt time.Time
}

// ProjectMembersCursor holds info for project members cursor pagination.
type ProjectMembersCursor struct {
	Search         string
	Limit          uint
	Page           uint
	Order          ProjectMemberOrder
	OrderDirection OrderDirection
}

// ProjectMembersPage represents a page of project members and invitations.
type ProjectMembersPage struct {
	ProjectMembers     []ProjectMember
	ProjectInvitations []ProjectInvitation

	Search         string
	Limit          uint
	Order          ProjectMemberOrder
	OrderDirection OrderDirection
	Offset         uint64
	PageCount      uint
	CurrentPage    uint
	TotalCount     uint64
}

// ProjectMemberOrder is used for querying project members in specified order.
type ProjectMemberOrder int8

const (
	// Name indicates that we should order by full name.
	Name ProjectMemberOrder = 1
	// Email indicates that we should order by email.
	Email ProjectMemberOrder = 2
	// Created indicates that we should order by created date.
	Created ProjectMemberOrder = 3
)

// ProjectMemberRole is used to indicate project member's role in the project.
type ProjectMemberRole int

const (
	// RoleAdmin indicates that the member has admin rights.
	RoleAdmin ProjectMemberRole = 0
	// RoleMember indicates that the member has regular member rights.
	RoleMember ProjectMemberRole = 1
)

func (mr ProjectMemberRole) String() string {
	switch mr {
	case RoleAdmin:
		return "admin"
	case RoleMember:
		return "member"
	}

	return ""
}
