// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/storj/pkg/satellite"

	"github.com/skyrings/skyring-common/tools/uuid"
	"storj.io/storj/pkg/satellite/satellitedb/dbx"
)

// implementation of User interface repository using spacemonkeygo/dbx orm
type users struct {
	db  *dbx.DB
	ctx context.Context
}

// NewUserRepository is a constructor for user repository
func NewUserRepository(ctx context.Context, db *dbx.DB) satellite.Users {
	return &users{
		db:  db,
		ctx: ctx,
	}
}

// Method for querying user by id from the database.
func (u *users) Get(id uuid.UUID) (*satellite.User, error) {

	userID := dbx.User_Id(id.String())

	user, err := u.db.Get_User_By_Id(u.ctx, userID)

	if err != nil {
		return nil, err
	}

	return satellite.UserFromDBX(user)
}

// Method for querying user by credentials from the database.
func (u *users) GetByCredentials(password []byte, email string) (*satellite.User, error) {

	userEmail := dbx.User_Email(email)
	userPassword := dbx.User_PasswordHash(password)

	user, err := u.db.Get_User_By_Email_And_PasswordHash(u.ctx, userEmail, userPassword)

	if err != nil {
		return nil, err
	}

	return satellite.UserFromDBX(user)
}

// Method for inserting user into the database
func (u *users) Insert(user *satellite.User) error {

	userID := dbx.User_Id(user.ID.String())
	userFirstName := dbx.User_FirstName(user.FirstName)
	userLastName := dbx.User_LastName(user.LastName)
	userEmail := dbx.User_Email(user.Email)
	userPasswordHash := dbx.User_PasswordHash(user.PasswordHash)

	_, err := u.db.Create_User(u.ctx, userID, userFirstName, userLastName, userEmail, userPasswordHash)

	return err
}

// Method for deleting user by Id from the database.
func (u *users) Delete(id uuid.UUID) error {

	userID := dbx.User_Id(id.String())

	_, err := u.db.Delete_User_By_Id(u.ctx, userID)

	return err
}

// Method for updating user entity
func (u *users) Update(user *satellite.User) error {

	userID := dbx.User_Id(user.ID.String())

	updateFields := dbx.User_Update_Fields{
		FirstName:    dbx.User_FirstName(user.FirstName),
		LastName:     dbx.User_LastName(user.LastName),
		Email:        dbx.User_Email(user.Email),
		PasswordHash: dbx.User_PasswordHash(user.PasswordHash),
	}

	_, err := u.db.Update_User_By_Id(u.ctx, userID, updateFields)

	return err
}
