// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/satellite"
	"storj.io/storj/pkg/satellite/satellitedb/dbx"
)

func TestUserRepository(t *testing.T) {
	//testing constants
	const (
		lastName    = "lastName"
		email       = "email@ukr.net"
		passValid   = "123456"
		name        = "name"
		newName     = "newName"
		newLastName = "newLastName"
		newEmail    = "newEmail@ukr.net"
		newPass     = "newPass1234567890123456789012345"
	)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// creating in-memory db and opens connection
	// to test with real db3 file use this connection string - "../db/accountdb.db3"
	db, err := New("sqlite3", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(db.Close)

	// creating tables
	err = db.CreateTables()
	if err != nil {
		t.Fatal(err)
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

		_, err = repository.Insert(ctx, user)

		assert.Nil(t, err)
		assert.NoError(t, err)
	})

	t.Run("Can't insert user with same email twice", func(t *testing.T) {
		user := &satellite.User{
			FirstName:    name,
			LastName:     lastName,
			Email:        email,
			PasswordHash: []byte(passValid),
			CreatedAt:    time.Now(),
		}

		_, err = repository.Insert(ctx, user)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Get user success", func(t *testing.T) {
		userByCreds, err := repository.GetByCredentials(ctx, []byte(passValid), email)

		assert.Equal(t, userByCreds.FirstName, name)
		assert.Equal(t, userByCreds.LastName, lastName)
		assert.Nil(t, err)
		assert.NoError(t, err)

		userByID, err := repository.Get(ctx, userByCreds.ID)

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

		assert.NoError(t, err)

		newUser := &satellite.User{
			ID:           oldUser.ID,
			FirstName:    newName,
			LastName:     newLastName,
			Email:        newEmail,
			PasswordHash: []byte(newPass),
		}

		err = repository.Update(ctx, newUser)

		assert.Nil(t, err)
		assert.NoError(t, err)

		newUser, err = repository.Get(ctx, oldUser.ID)

		assert.NoError(t, err)

		assert.Equal(t, newUser.ID, oldUser.ID)
		assert.Equal(t, newUser.FirstName, newName)
		assert.Equal(t, newUser.LastName, newLastName)
		assert.Equal(t, newUser.Email, newEmail)
		assert.Equal(t, newUser.PasswordHash, []byte(newPass))
		assert.Equal(t, newUser.CreatedAt, oldUser.CreatedAt)
	})

	t.Run("Delete user success", func(t *testing.T) {
		oldUser, err := repository.GetByCredentials(ctx, []byte(newPass), newEmail)

		assert.NoError(t, err)

		err = repository.Delete(ctx, oldUser.ID)

		assert.Nil(t, err)
		assert.NoError(t, err)

		_, err = repository.Get(ctx, oldUser.ID)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})
}

func TestUserFromDbx(t *testing.T) {
	t.Run("can't create dbo from nil dbx model", func(t *testing.T) {
		user, err := userFromDBX(nil)

		assert.Nil(t, user)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("can't create dbo from dbx model with invalid ID", func(t *testing.T) {
		dbxUser := dbx.User{
			Id:           []byte("qweqwe"),
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
