// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"net/mail"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// Users exposes methods to manage User table in database.
type Users interface {
	// Get is a method for querying user from the database by id.
	Get(ctx context.Context, id uuid.UUID) (*User, error)
	// GetByEmail is a method for querying user by email from the database.
	GetByEmail(ctx context.Context, email string) (*User, error)
	// Insert is a method for inserting user into the database.
	Insert(ctx context.Context, user *User) (*User, error)
	// Delete is a method for deleting user by Id from the database.
	Delete(ctx context.Context, id uuid.UUID) error
	// Update is a method for updating user entity.
	Update(ctx context.Context, user *User) error
}

// UserInfo holds User updatable data.
type UserInfo struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

// IsValid checks UserInfo validity and returns error describing whats wrong.
func (user *UserInfo) IsValid() error {
	var errs validationErrors

	// validate email
	_, err := mail.ParseAddress(user.Email)
	errs.AddWrap(err)

	// validate firstName
	if user.FirstName == "" {
		errs.Add("firstName can't be empty")
	}

	return errs.Combine()
}

// CreateUser struct holds info for User creation.
type CreateUser struct {
	UserInfo
	Password string `json:"password"`
}

// IsValid checks CreateUser validity and returns error describing whats wrong.
func (user *CreateUser) IsValid() error {
	var errs validationErrors

	errs.AddWrap(user.UserInfo.IsValid())
	errs.AddWrap(validatePassword(user.Password))

	return errs.Combine()
}

// User is a database object that describes User entity.
type User struct {
	ID uuid.UUID `json:"id"`

	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`

	Email        string `json:"email"`
	PasswordHash []byte `json:"passwordHash"`

	CreatedAt time.Time `json:"createdAt"`
}
