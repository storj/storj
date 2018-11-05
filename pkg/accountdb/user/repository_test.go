// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package user

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/accountdb"
	"testing"
	"time"
)

// test values
var (
	lastName = "lastName"
	email = "email@ukr.net"
	pass = "pass"
	passValid = "123456"
	name = "name"
	newName = "newName"
	newLastName = "newLastName"
	newEmail = "newEmail@ukr.net"
	newPass = "newPass"
)

func TestRepository(t *testing.T) {

	accountdb.NewDbFactory("file::memory:?mode=memory&cache=shared", "sqlite3")

	db, err := accountdb.GetDb()

	if err != nil {

	}

	defer func() {
		conn, _ := db.GetConnection()
		conn.Close()
	}()

	repository := NewRepository()

	cases := []struct {
		testName, address string
		testFunc          func()
	}{
		{
			testName: "Table created",

			testFunc: func() {
				err := repository.CreateTable()

				assert.Nil(t, err)
				assert.NoError(t, err)
			},
		},
		{
			testName: "Can't insert user with empty FirstName",

			testFunc: func() {
				user := NewUser(uuid.New(), "", lastName, email, passValid, time.Now())

				err := repository.Insert(user)

				assert.NotNil(t, err)
				assert.Error(t, err)
			},
		},
		{
			testName: "Can't insert user with empty LastName",

			testFunc: func() {
				user := NewUser(uuid.New(), name, "", email, passValid, time.Now())

				err := repository.Insert(user)

				assert.NotNil(t, err)
				assert.Error(t, err)
			},
		},
		{
			testName: "Can't insert user with empty Email",

			testFunc: func() {
				user := NewUser(uuid.New(), name, lastName, "", passValid, time.Now())

				err := repository.Insert(user)

				assert.NotNil(t, err)
				assert.Error(t, err)
			},
		},
		{
			testName: "Can't insert user with empty Password",

			testFunc: func() {
				user := NewUser(uuid.New(), name, lastName, email, "", time.Now())

				err := repository.Insert(user)

				assert.NotNil(t, err)
				assert.Error(t, err)
			},
		},
		{
			testName: "Can't insert user with length(Password) < 6",

			testFunc: func() {
				user := NewUser(uuid.New(), name, lastName, email, pass, time.Now())

				err := repository.Insert(user)

				assert.NotNil(t, err)
				assert.Error(t, err)
			},
		},
		{
			testName: "User insertion success",

			testFunc: func() {
				user := NewUser(uuid.New(), name, lastName, email, passValid, time.Now())

				err := repository.Insert(user)

				assert.Nil(t, err)
				assert.NoError(t, err)
			},
		},
		{
			testName: "Can't insert user with same email twice",

			testFunc: func() {
				user := NewUser(uuid.New(), name, lastName, email, passValid, time.Now())

				err := repository.Insert(user)

				assert.NotNil(t, err)
				assert.Error(t, err)
			},
		},
		{
			testName: "Get user success",

			testFunc: func() {
				userByCreds, err := repository.GetByCredentials(passValid, email)

				assert.Equal(t, userByCreds.firstName, name)
				assert.Equal(t, userByCreds.lastName, lastName)
				assert.Nil(t, err)
				assert.NoError(t, err)

				userById, err := repository.GetByCredentials(passValid, email)

				assert.Equal(t, userById.firstName, name)
				assert.Equal(t, userById.lastName, lastName)
				assert.Nil(t, err)
				assert.NoError(t, err)

				assert.Equal(t, userById.Id(), userByCreds.Id())
				assert.Equal(t, userById.firstName, userByCreds.firstName)
				assert.Equal(t, userById.lastName, userByCreds.lastName)
				assert.Equal(t, userById.email, userByCreds.email)
				assert.Equal(t, userById.password, userByCreds.password)
				assert.Equal(t, userById.CreationDate(), userByCreds.CreationDate())
			},
		},
		{
			testName: "Update user success",

			testFunc: func() {
				oldUser, err := repository.GetByCredentials(passValid, email)

				err = repository.Update(oldUser.Id(), newName, newLastName, newEmail)

				assert.Nil(t, err)
				assert.NoError(t, err)

				newUser, err := repository.Get(oldUser.Id())

				assert.Equal(t, newUser.Id(), oldUser.Id())
				assert.Equal(t, newUser.firstName, newName)
				assert.Equal(t, newUser.lastName, newLastName)
				assert.Equal(t, newUser.email, newEmail)
				assert.Equal(t, newUser.password, oldUser.password)
				assert.Equal(t, newUser.CreationDate(), oldUser.CreationDate())
			},
		},
		{
			testName: "Update password success",

			testFunc: func() {
				oldUser, err := repository.GetByCredentials(passValid, newEmail)

				err = repository.UpdatePassword(oldUser.Id(), newPass)

				assert.Nil(t, err)
				assert.NoError(t, err)

				newUser, err := repository.Get(oldUser.Id())

				assert.Equal(t, newUser.Id(), oldUser.Id())
				assert.Equal(t, newUser.firstName, oldUser.firstName)
				assert.Equal(t, newUser.lastName, oldUser.lastName)
				assert.Equal(t, newUser.email, oldUser.email)
				assert.Equal(t, newUser.password, newPass)
				assert.Equal(t, newUser.CreationDate(), oldUser.CreationDate())
			},
		},
		{
			testName: "Delete user success",

			testFunc: func() {
				oldUser, err := repository.GetByCredentials(newPass, newEmail)

				err = repository.Delete(oldUser.Id())

				assert.Nil(t, err)
				assert.NoError(t, err)

				_, err = repository.Get(oldUser.Id())

				assert.NotNil(t, err)
				assert.Error(t, err)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			c.testFunc()
		})
	}
}