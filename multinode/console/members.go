// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

// Members exposes needed by MND MembersDB functionality.
//
// architecture: Database
type Members interface {
	// Invite will create empty row in membersDB.
	Invite(ctx context.Context, member Member) error
	// Update updates all updatable fields of member.
	Update(ctx context.Context, member Member) error
	// Remove deletes member from membersDB.
	Remove(ctx context.Context, id uuid.UUID) error
	// GetByEmail will return member with specified email.
	GetByEmail(ctx context.Context, email string) (Member, error)
	// GetByID will return member with specified id.
	GetByID(ctx context.Context, id uuid.UUID) (Member, error)
}

// ErrNoMember is a special error type that indicates about absence of member in MembersDB.
var ErrNoMember = errs.Class("no such member")

// Member represents some person that is invited to the MND by node owner.
// Member will have configurable access privileges that will define which functions and which nodes are available for him.
type Member struct {
	ID           uuid.UUID
	Email        string
	Name         string
	PasswordHash []byte
}
