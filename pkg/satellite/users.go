// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// Users exposes methods to manage User table in database.
type Users interface {
	// GetByCredentials is a method for querying user by credentials from the database.
	GetByCredentials(ctx context.Context, password []byte, email string) (*User, error)
	// Get is a method for querying user from the database by id
	Get(ctx context.Context, id uuid.UUID) (*User, error)
	// Insert is a method for inserting user into the database
	Insert(ctx context.Context, user *User) (*User, error)
	// Delete is a method for deleting user by Id from the database.
	Delete(ctx context.Context, id uuid.UUID) error
	// Update is a method for updating user entity
	Update(ctx context.Context, user *User) error
}

// User is a database object that describes User entity
type User struct {
	ID uuid.UUID

	FirstName    string
	LastName     string
	Email        string
	PasswordHash []byte

	CreatedAt time.Time
}
