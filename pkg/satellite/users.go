// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"net/mail"
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

// UserInfo holds data needed to create/update User
type UserInfo struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

// IsValid checks UserInfo validity and returns error describing whats wrong
func (user *UserInfo) IsValid() error {
	var errs validationErrors

	// validate email
	_, err := mail.ParseAddress(user.Email)
	errs.AddWrap(err)

	// validate firstName
	if user.FirstName == "" {
		errs.Add("firstName can't be empty")
	}

	{ // password checks
		if len(user.Password) < passMinLength {
			errs.Add("password can't be less than %d characters", passMinLength)
		}

		if countNumerics(user.Password) < passMinNumberCount {
			errs.Add("password should contain at least %d digits", passMinNumberCount)
		}

		if countLetters(user.Password) < passMinAZCount {
			errs.Add("password should contain at least %d alphabetic characters", passMinAZCount)
		}
	}

	return errs.Combine()
}

// User is a database object that describes User entity
type User struct {
	ID uuid.UUID `json:"id"`

	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	Email        string `json:"email"`
	PasswordHash []byte `json:"passwordHash"`

	CreatedAt time.Time `json:"createdAt"`
}
