// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/satellite/satellitedb/dbx"
	"testing"
	"time"

	"storj.io/storj/pkg/satellite"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRepository(t *testing.T) {

	//testing constants
	const (
		lastName    = "lastName"
		email       = "email@ukr.net"
		passValid   = "123456"
		name        = "name"
		newName     = "newName"
		newLastName = "newLastName"
		newEmail    = "newEmail@ukr.net"
		newPass     = "newPass"
	)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// to test with real db3 file use this connection string - "../db/accountdb.db3"
	db, err := New("sqlite3", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		assert.NoError(t, err)
	}
	defer ctx.Check(db.Close)

	err = db.CreateTables()
	if err != nil {
		assert.NoError(t, err)
	}

	repository := db.Users()

	t.Run("User insertion success", func(t *testing.T) {

		id, err := uuid.New()

		if err != nil {
			assert.NoError(t, err)
		}

		user := &satellite.User{
			ID:           *id,
			FirstName:    name,
			LastName:     lastName,
			Email:        email,
			PasswordHash: []byte(passValid),
			CreatedAt:    time.Now(),
		}

		err = repository.Insert(ctx, user)

		assert.Nil(t, err)
		assert.NoError(t, err)
	})

	t.Run("Can't insert user with same email twice", func(t *testing.T) {

		id, err := uuid.New()

		if err != nil {
			assert.NoError(t, err)
		}

		user := &satellite.User{
			ID:           *id,
			FirstName:    name,
			LastName:     lastName,
			Email:        email,
			PasswordHash: []byte(passValid),
			CreatedAt:    time.Now(),
		}

		err = repository.Insert(ctx, user)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Get user success", func(t *testing.T) {
		userByCreds, err := repository.GetByCredentials(ctx, []byte(passValid), email)

		assert.Equal(t, userByCreds.FirstName, name)
		assert.Equal(t, userByCreds.LastName, lastName)
		assert.Nil(t, err)
		assert.NoError(t, err)

		userByID, err := repository.GetByCredentials(ctx, []byte(passValid), email)

		assert.Equal(t, userByID.FirstName, name)
		assert.Equal(t, userByID.LastName, lastName)
		assert.Nil(t, err)
		assert.NoError(t, err)

		assert.Equal(t, userByID.ID, userByCreds.ID)
		assert.Equal(t, userByID.FirstName, userByCreds.FirstName)
		assert.Equal(t, userByID.LastName, userByCreds.LastName)
		assert.Equal(t, userByID.Email, userByCreds.Email)
		assert.Equal(t, userByID.PasswordHash, userByCreds.PasswordHash)
		assert.Equal(t, userByID.CreatedAt, userByCreds.CreatedAt)
	})

	t.Run("Update user success", func(t *testing.T) {
		oldUser, err := repository.GetByCredentials(ctx, []byte(passValid), email)

		if err != nil {
			assert.NoError(t, err)
		}

		newUser := &satellite.User{
			ID:           oldUser.ID,
			FirstName:    newName,
			LastName:     newLastName,
			Email:        newEmail,
			PasswordHash: []byte(newPass),
			CreatedAt:    oldUser.CreatedAt,
		}

		err = repository.Update(ctx, newUser)

		assert.Nil(t, err)
		assert.NoError(t, err)

		newUser, err = repository.Get(ctx, oldUser.ID)

		if err != nil {
			assert.NoError(t, err)
		}

		assert.Equal(t, newUser.ID, oldUser.ID)
		assert.Equal(t, newUser.FirstName, newName)
		assert.Equal(t, newUser.LastName, newLastName)
		assert.Equal(t, newUser.Email, newEmail)
		assert.Equal(t, newUser.PasswordHash, []byte(newPass))
		assert.Equal(t, newUser.CreatedAt, oldUser.CreatedAt)
	})

	t.Run("Delete user success", func(t *testing.T) {
		oldUser, err := repository.GetByCredentials(ctx, []byte(newPass), newEmail)

		if err != nil {
			assert.NoError(t, err)
		}

		err = repository.Delete(ctx, oldUser.ID)

		assert.Nil(t, err)
		assert.NoError(t, err)

		_, err = repository.Get(ctx, oldUser.ID)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})
}

func TestUserDboFromDbx(t *testing.T) {

	t.Run("can't create dbo from nil dbx model", func(t *testing.T) {
		user, err := userFromDBX(nil)

		assert.Nil(t, user)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("can't create dbo from dbx model with invalid Id", func(t *testing.T) {
		dbxUser := dbx.User{
			Id:           "qweqwe",
			FirstName:    "FirstName",
			LastName:     "LastName",
			Email:        "email@ukr.net",
			PasswordHash: []byte("ihqerfgnu238723huagsd"),
			CreatedAt:    time.Now(),
		}

		user, err := userFromDBX(&dbxUser)

		assert.Nil(t, user)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})
}