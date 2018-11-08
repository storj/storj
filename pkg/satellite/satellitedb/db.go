// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/storj/pkg/satellite"

	"storj.io/storj/pkg/satellite/satellitedb/dbx"
)

type database struct {
	db  *dbx.DB
	ctx context.Context

	userRepository satellite.Users
}

// NewDB - constructor for DB
func NewDB(ctx context.Context) (satellite.DB, error) {

	//TODO: place all string constants in better place
	db, err := dbx.Open("sqlite3", "../db/accountdb.db3")

	if err != nil {
		return nil, err
	}

	database := &database{
		db:  db,
		ctx: ctx,
	}

	return database, nil
}

// Getter for User repository
func (d *database) User() satellite.Users {
	if d.userRepository == nil {
		d.userRepository = NewUserRepository(d.ctx, d.db)
	}

	return d.userRepository
}

// Method for creating all tables for accountdb
func (d *database) CreateTables() error {

	_, err := d.db.Exec(d.db.Schema())

	return err
}
