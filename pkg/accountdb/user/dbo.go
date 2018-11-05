// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package user

import (
	"github.com/google/uuid"
	"storj.io/storj/pkg/accountdb"
	"time"
)

// Database object that describes User entity
type User struct {
	accountdb.BaseDbo

	firstName string
	lastName string
	email string
	password string
}

func (d User) FirstName() string {
	return d.firstName
}

func (d User) LastName() string {
	return d.lastName
}

func (d User) Email() string {
	return d.email
}

func (d User) Password() string {
	return d.password
}

func NewUser(id uuid.UUID, firstName, lastName, email, password string, creationDate time.Time) *User {
	return &User{
		accountdb.NewBaseDbo(id, creationDate),
		firstName,
		lastName,
		email,
		password,
	}
}