// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repositories

import (
	"context"
	"github.com/skyrings/skyring-common/tools/uuid"
	"storj.io/storj/pkg/accountdb/dbo"
	"storj.io/storj/pkg/accountdb/dbx"
)

// Exposes methods to manage User table in database.
type User interface {
	GetByCredentials(password, email string) (*dbo.User, error)
	Get(id uuid.UUID) (*dbo.User, error)
	Insert(user *dbo.User) (error)
	Delete(id uuid.UUID) (error)
	Update(user *dbo.User) (error)
}

// implementation of User interface repository using spacemonkeygo/dbx orm
type user struct {
	db *dbx.DB
	ctx context.Context
}

// Constructor for user repository
func NewUserRepository(db *dbx.DB, ctx context.Context) User {
	return &user{
		db,
		ctx,
	}
}

// Method for querying user by id from the database.
func (u *user) Get(id uuid.UUID) (*dbo.User, error) {

	result := &dbo.User{}

	userId := dbx.User_Id(id.String())

	user, err :=  u.db.Get_User_By_Id(u.ctx, userId)

	if err != nil {
		return nil, err
	}

	return result.FromDbx(user)
}

// Method for querying user by credentials from the database.
func (u *user) GetByCredentials(password, email string) (*dbo.User, error) {

	result := &dbo.User{}

	userEmail := dbx.User_Email(email)
	userPassword := dbx.User_PasswordHash(password)

	user, err :=  u.db.Get_User_By_Email_And_PasswordHash(u.ctx, userEmail, userPassword)

	if err != nil {
		return nil, err
	}

	return result.FromDbx(user)
}

// Method for inserting user into the database
func (u *user) Insert(user *dbo.User) (error) {

	userId := dbx.User_Id(user.Id().String())
	userFirstName := dbx.User_FirstName(user.FirstName())
	userLastName := dbx.User_LastName(user.LastName())
	userEmail := dbx.User_Email(user.Email())
	userPasswordHash := dbx.User_PasswordHash(user.Password())

	_, err := u.db.Create_User(u.ctx, userId, userFirstName, userLastName, userEmail, userPasswordHash)

	return err
}

// Method for deleting user by Id from the database.
func (u *user) Delete(id uuid.UUID) (error) {

	userId := dbx.User_Id(id.String())

	_, err := u.db.Delete_User_By_Id(u.ctx, userId)

	return err
}

// Method for updating user entity
func (u *user) Update(user *dbo.User) (error) {

	userId := dbx.User_Id(user.Id().String())
	updateFields := dbx.User_Update_Fields{
		dbx.User_FirstName(user.FirstName()),
		dbx.User_LastName(user.LastName()),
		dbx.User_Email(user.Email()),
		dbx.User_PasswordHash(user.Password()),
	}

	_, err := u.db.Update_User_By_Id(u.ctx, userId, updateFields)

	return err
}