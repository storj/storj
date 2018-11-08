// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accountdb

import (
	"context"
	"storj.io/storj/pkg/accounts"
	"storj.io/storj/pkg/accounts/accountdb/dbx"
	"storj.io/storj/pkg/accounts/accountdb/repositories"
)

type database struct {
	db *dbx.DB
	ctx context.Context

	userRepository accounts.Users
}

// NewDB - constructor for DB
func NewDB(ctx context.Context) (accounts.DB, error) {

	//TODO: place all string constants in better place
	db, err := dbx.Open("sqlite3", "../db/accountdb.db3")

	if err != nil {
		return nil, err
	}

	database := &database{
		db: db,
		ctx: ctx,
	}

	return database, nil
}

// Getter for User repository
func (uow *database) User() accounts.Users {
	if uow.userRepository == nil {
		uow.userRepository = repositories.NewUserRepository(uow.ctx, uow.db)
	}

	return uow.userRepository
}

// Method for creating all tables for accountdb
func (uow *database) CreateTables() error {

	_, err := uow.db.Exec(uow.db.Schema())

	return err
}

// Closing connection
func (uow *database) Dispose() error {
	return uow.db.Close()
}
