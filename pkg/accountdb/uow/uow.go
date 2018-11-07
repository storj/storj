// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uow

import (
	"context"
	"storj.io/storj/pkg/accountdb/dbx"
	"storj.io/storj/pkg/accountdb/repositories"
)

//TODO: add here all future created repositories, add unit tests
// Used to manage db connections and context through different repositories
type UnitOfWork interface {
	// methods to get concrete repository
	User() repositories.User
	// Company() repositories.Company
	// Project() repositories.Project

	CreateTables() error

	Dispose() error
}

type uow struct {
	db *dbx.DB
	ctx context.Context

	// repositories section
	userRepository repositories.User
}

// Constructor for UnitOfWork
func NewUnitOfWork(ctx context.Context) (UnitOfWork, error) {

	//TODO: place all string constants in better place
	db, err := dbx.Open("sqlite3", "../db/accountdb.db3")

	if err != nil {
		return nil, err
	}

	unitOfWork := &uow{
		db: db,
		ctx: ctx,
	}

	return unitOfWork, nil
}

// Getter for User repository
func (uow *uow) User() repositories.User {
	if uow.userRepository == nil {
		uow.userRepository = repositories.NewUserRepository(uow.db, uow.ctx)
	}

	return uow.userRepository
}

// Method for creating all tables for accountdb
func (uow *uow) CreateTables() error {

	_, err := uow.db.Exec(uow.db.Schema())

	return err
}

// Closing connection
func (uow *uow) Dispose() error {
	return uow.db.Close()
}