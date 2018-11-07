// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repositories

import (
	"context"
	"fmt"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/accountdb/dbo"
	"storj.io/storj/pkg/accountdb/dbx"
	"testing"
	"time"
)

// test values
var (
	lastName = "lastName"
	email = "email@ukr.net"
	passValid = "123456"
	name = "name"
	newName = "newName"
	newLastName = "newLastName"
	newEmail = "newEmail@ukr.net"
	newPass = "newPass"
)

func TestRepository(t *testing.T) {

	// to test with real db3 file user this connection string - "../db/accountdb.db3"
	db, err := dbx.Open("sqlite3", "file::memory:?mode=memory&cache=shared")

	if err != nil {
		fmt.Println(err)
	}

	_, err = db.Exec(db.Schema())

	if err != nil {
		fmt.Println(err)
	}

	defer func() {
		db.Close()
	}()

	repository := NewUserRepository(db, context.Background())

	t.Run("User insertion success", func(t *testing.T) {

		id, err := uuid.New()

		if err != nil {
			fmt.Println(err)
		}

		user := dbo.NewUser(*id, name, lastName, email, passValid, time.Now())

		err = repository.Insert(user)

		assert.Nil(t, err)
		assert.NoError(t, err)
	})


	t.Run("Can't insert user with same email twice", func(t *testing.T) {

		id, err := uuid.New()

		if err != nil {
			fmt.Println(err)
		}

		user := dbo.NewUser(*id, name, lastName, email, passValid, time.Now())

		err = repository.Insert(user)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Get user success", func(t *testing.T) {
		userByCreds, err := repository.GetByCredentials(passValid, email)

		assert.Equal(t, userByCreds.FirstName(), name)
		assert.Equal(t, userByCreds.LastName(), lastName)
		assert.Nil(t, err)
		assert.NoError(t, err)

		userById, err := repository.GetByCredentials(passValid, email)

		assert.Equal(t, userById.FirstName(), name)
		assert.Equal(t, userById.LastName(), lastName)
		assert.Nil(t, err)
		assert.NoError(t, err)

		assert.Equal(t, userById.Id(), userByCreds.Id())
		assert.Equal(t, userById.FirstName(), userByCreds.FirstName())
		assert.Equal(t, userById.LastName(), userByCreds.LastName())
		assert.Equal(t, userById.Email(), userByCreds.Email())
		assert.Equal(t, userById.Password(), userByCreds.Password())
		assert.Equal(t, userById.CreatedAt(), userByCreds.CreatedAt())
	})

	t.Run("Update user success", func(t *testing.T) {
		oldUser, err := repository.GetByCredentials(passValid, email)
		newUser := dbo.NewUser(oldUser.Id(), newName, newLastName, newEmail, newPass, oldUser.CreatedAt())

		err = repository.Update(newUser)

		assert.Nil(t, err)
		assert.NoError(t, err)

		newUser, err = repository.Get(oldUser.Id())

		assert.Equal(t, newUser.Id(), oldUser.Id())
		assert.Equal(t, newUser.FirstName(), newName)
		assert.Equal(t, newUser.LastName(), newLastName)
		assert.Equal(t, newUser.Email(), newEmail)
		assert.Equal(t, newUser.Password(), newPass)
		assert.Equal(t, newUser.CreatedAt(), oldUser.CreatedAt())
	})

	t.Run("Delete user success", func(t *testing.T) {
		oldUser, err := repository.GetByCredentials(newPass, newEmail)

		err = repository.Delete(oldUser.Id())

		assert.Nil(t, err)
		assert.NoError(t, err)

		_, err = repository.Get(oldUser.Id())

		assert.NotNil(t, err)
		assert.Error(t, err)
	})
}