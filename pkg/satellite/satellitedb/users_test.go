// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"storj.io/storj/pkg/satellite"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/satellite/satellitedb/dbx"
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

	// to test with real db3 file use this connection string - "../db/accountdb.db3"
	db, err := dbx.Open("sqlite3", "file::memory:?mode=memory&cache=shared")

	if err != nil {
		fmt.Println(err)
	}

	_, err = db.Exec(db.Schema())

	if err != nil {
		fmt.Println(err)
	}

	defer func() {
		err := db.Close()

		if err != nil {
			fmt.Println(err)
		}
	}()

	repository := NewUserRepository(context.Background(), db)

	t.Run("User insertion success", func(t *testing.T) {

		id, err := uuid.New()

		if err != nil {
			fmt.Println(err)
		}

		user := &satellite.User{
			ID:           *id,
			FirstName:    name,
			LastName:     lastName,
			Email:        email,
			PasswordHash: []byte(passValid),
			CreatedAt:    time.Now(),
		}

		err = repository.Insert(user)

		assert.Nil(t, err)
		assert.NoError(t, err)
	})

	t.Run("Can't insert user with same email twice", func(t *testing.T) {

		id, err := uuid.New()

		if err != nil {
			fmt.Println(err)
		}

		user := &satellite.User{
			ID:           *id,
			FirstName:    name,
			LastName:     lastName,
			Email:        email,
			PasswordHash: []byte(passValid),
			CreatedAt:    time.Now(),
		}

		err = repository.Insert(user)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Get user success", func(t *testing.T) {
		userByCreds, err := repository.GetByCredentials([]byte(passValid), email)

		assert.Equal(t, userByCreds.FirstName, name)
		assert.Equal(t, userByCreds.LastName, lastName)
		assert.Nil(t, err)
		assert.NoError(t, err)

		userByID, err := repository.GetByCredentials([]byte(passValid), email)

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
		oldUser, err := repository.GetByCredentials([]byte(passValid), email)

		if err != nil {
			fmt.Println(err)
			t.Fail()
		}

		newUser := &satellite.User{
			ID:           oldUser.ID,
			FirstName:    newName,
			LastName:     newLastName,
			Email:        newEmail,
			PasswordHash: []byte(newPass),
			CreatedAt:    oldUser.CreatedAt,
		}

		err = repository.Update(newUser)

		assert.Nil(t, err)
		assert.NoError(t, err)

		newUser, err = repository.Get(oldUser.ID)

		if err != nil {
			fmt.Println(err)
		}

		assert.Equal(t, newUser.ID, oldUser.ID)
		assert.Equal(t, newUser.FirstName, newName)
		assert.Equal(t, newUser.LastName, newLastName)
		assert.Equal(t, newUser.Email, newEmail)
		assert.Equal(t, newUser.PasswordHash, []byte(newPass))
		assert.Equal(t, newUser.CreatedAt, oldUser.CreatedAt)
	})

	t.Run("Delete user success", func(t *testing.T) {
		oldUser, err := repository.GetByCredentials([]byte(newPass), newEmail)

		if err != nil {
			fmt.Println(err)
		}

		err = repository.Delete(oldUser.ID)

		assert.Nil(t, err)
		assert.NoError(t, err)

		_, err = repository.Get(oldUser.ID)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})
}
