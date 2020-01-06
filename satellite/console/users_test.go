// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

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

func TestUserRepository(t *testing.T) {

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		repository := db.Console().Users()
		partnerID := testrand.UUID()

		// Test with and without partnerID
		user := &console.User{
			ID:           testrand.UUID(),
			FullName:     name,
			ShortName:    lastName,
			Email:        email,
			PartnerID:    partnerID,
			PasswordHash: []byte(passValid),
			CreatedAt:    time.Now(),
		}
		testUsers(ctx, t, repository, user)

		user = &console.User{
			ID:           testrand.UUID(),
			FullName:     name,
			ShortName:    lastName,
			Email:        email,
			PasswordHash: []byte(passValid),
			CreatedAt:    time.Now(),
		}
		testUsers(ctx, t, repository, user)
	})
}

func TestUserEmailCase(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		for _, testCase := range []struct {
			email string
		}{
			{email: "prettyandsimple@example.com"},
			{email: "firstname.lastname@domain.com	"},
			{email: "email@subdomain.domain.com	"},
			{email: "firstname+lastname@domain.com	"},
			{email: "email@[123.123.123.123]	"},
			{email: "\"email\"@domain.com"},
			{email: "_______@domain.com	"},
		} {
			newUser := &console.User{
				ID:           testrand.UUID(),
				FullName:     newName,
				ShortName:    newLastName,
				Email:        testCase.email,
				Status:       console.Active,
				PasswordHash: []byte(newPass),
			}

			createdUser, err := db.Console().Users().Insert(ctx, newUser)
			assert.NoError(t, err)
			assert.Equal(t, testCase.email, createdUser.Email)

			createdUser.Status = console.Active

			err = db.Console().Users().Update(ctx, createdUser)
			assert.NoError(t, err)

			retrievedUser, err := db.Console().Users().GetByEmail(ctx, testCase.email)
			assert.NoError(t, err)
			assert.Equal(t, testCase.email, retrievedUser.Email)
		}
	})
}

func testUsers(ctx context.Context, t *testing.T, repository console.Users, user *console.User) {

	t.Run("User insertion success", func(t *testing.T) {

		insertedUser, err := repository.Insert(ctx, user)
		assert.NoError(t, err)

		insertedUser.Status = console.Active

		err = repository.Update(ctx, insertedUser)
		assert.NoError(t, err)
	})

	t.Run("Get user success", func(t *testing.T) {
		userByEmail, err := repository.GetByEmail(ctx, email)
		assert.NoError(t, err)
		assert.Equal(t, name, userByEmail.FullName)
		assert.Equal(t, lastName, userByEmail.ShortName)
		assert.Equal(t, user.PartnerID, userByEmail.PartnerID)

		userByID, err := repository.Get(ctx, userByEmail.ID)
		assert.NoError(t, err)
		assert.Equal(t, name, userByID.FullName)
		assert.Equal(t, lastName, userByID.ShortName)
		assert.Equal(t, user.PartnerID, userByID.PartnerID)

		assert.Equal(t, userByID.ID, userByEmail.ID)
		assert.Equal(t, userByID.FullName, userByEmail.FullName)
		assert.Equal(t, userByID.ShortName, userByEmail.ShortName)
		assert.Equal(t, userByID.Email, userByEmail.Email)
		assert.Equal(t, userByID.PasswordHash, userByEmail.PasswordHash)
		assert.Equal(t, userByID.PartnerID, userByEmail.PartnerID)
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
		assert.Equal(t, oldUser.ID, newUser.ID)
		assert.Equal(t, newName, newUser.FullName)
		assert.Equal(t, newLastName, newUser.ShortName)
		assert.Equal(t, newEmail, newUser.Email)
		assert.Equal(t, []byte(newPass), newUser.PasswordHash)
		// PartnerID should not change
		assert.Equal(t, user.PartnerID, newUser.PartnerID)
		assert.Equal(t, oldUser.CreatedAt, newUser.CreatedAt)
	})

	t.Run("Delete user success", func(t *testing.T) {
		oldUser, err := repository.GetByEmail(ctx, newEmail)
		assert.NoError(t, err)

		err = repository.Delete(ctx, oldUser.ID)
		assert.NoError(t, err)

		_, err = repository.Get(ctx, oldUser.ID)
		assert.Error(t, err)
	})
}
