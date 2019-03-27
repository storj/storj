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
	FullName  string `json:"fullName"`
	ShortName string `json:"shortName"`
	Email     string `json:"email"`
}

// IsValid checks UserInfo validity and returns error describing whats wrong.
func (user *UserInfo) IsValid() error {
	var errs validationErrors

	// validate email
	_, err := mail.ParseAddress(user.Email)
	errs.AddWrap(err)

	// validate fullName
	if user.FullName == "" {
		errs.Add("fullName can't be empty")
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

// UserStatus - is used to indicate status of the users account
type UserStatus int

const (
	// Inactive is a user status that he receives after registration
	Inactive UserStatus = 0
	// Active is a user status that he receives after account activation
	Active UserStatus = 1
	// Deleted is a user status that he receives after deleting account
	Deleted UserStatus = 2
)

// User is a database object that describes User entity.
type User struct {
	ID uuid.UUID `json:"id"`

	FullName  string `json:"fullName"`
	ShortName string `json:"shortName"`

	Email        string `json:"email"`
	PasswordHash []byte `json:"passwordHash"`

	Status UserStatus `json:"status"`

	CreatedAt time.Time `json:"createdAt"`
}
