// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUserRepository(t *testing.T) {
	//testing constants
	const (
		lastName    = "lastName"
		email       = "email@mail.test"
		passValid   = "123456"
		name        = "name"
		newName     = "newName"
		newLastName = "newLastName"
		newEmail    = "newEmail@mail.test"
		newPass     = "newPass1234567890123456789012345"
	)

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		repository := db.Console().Users()

		t.Run("User insertion success", func(t *testing.T) {
			user := &console.User{
				ID:           testrand.UUID(),
				FullName:     name,
				ShortName:    lastName,
				Email:        email,
				PasswordHash: []byte(passValid),
				CreatedAt:    time.Now(),
			}

			insertedUser, err := repository.Insert(ctx, user)
			assert.NoError(t, err)

			insertedUser.Status = console.Active

			err = repository.Update(ctx, insertedUser)
			assert.NoError(t, err)
		})

		t.Run("Get user success", func(t *testing.T) {
			userByEmail, err := repository.GetByEmail(ctx, email)
			assert.Equal(t, userByEmail.FullName, name)
			assert.Equal(t, userByEmail.ShortName, lastName)
			assert.NoError(t, err)

			userByID, err := repository.Get(ctx, userByEmail.ID)
			assert.Equal(t, userByID.FullName, name)
			assert.Equal(t, userByID.ShortName, lastName)
			assert.NoError(t, err)

			assert.Equal(t, userByID.ID, userByEmail.ID)
			assert.Equal(t, userByID.FullName, userByEmail.FullName)
			assert.Equal(t, userByID.ShortName, userByEmail.ShortName)
			assert.Equal(t, userByID.Email, userByEmail.Email)
			assert.Equal(t, userByID.PasswordHash, userByEmail.PasswordHash)
			assert.Equal(t, userByID.CreatedAt, userByEmail.CreatedAt)
		})

		t.Run("Update user success", func(t *testing.T) {
			oldUser, err := repository.GetByEmail(ctx, email)
			assert.NoError(t, err)

			newUser := &console.User{
				ID:           oldUser.ID,
				FullName:     newName,
				ShortName:    newLastName,
				Email:        newEmail,
				Status:       console.Active,
				PasswordHash: []byte(newPass),
			}

			err = repository.Update(ctx, newUser)
			assert.NoError(t, err)

			newUser, err = repository.Get(ctx, oldUser.ID)
			assert.NoError(t, err)
			assert.Equal(t, newUser.ID, oldUser.ID)
			assert.Equal(t, newUser.FullName, newName)
			assert.Equal(t, newUser.ShortName, newLastName)
			assert.Equal(t, newUser.Email, newEmail)
			assert.Equal(t, newUser.PasswordHash, []byte(newPass))
			assert.Equal(t, newUser.CreatedAt, oldUser.CreatedAt)
		})

		t.Run("Delete user success", func(t *testing.T) {
			oldUser, err := repository.GetByEmail(ctx, newEmail)
			assert.NoError(t, err)

			err = repository.Delete(ctx, oldUser.ID)
			assert.NoError(t, err)

			_, err = repository.Get(ctx, oldUser.ID)
			assert.Error(t, err)
		})
	})
}
