// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accountdb

import (
	"database/sql"
)

// Exposes base methods to manage entities in database.
type BaseRepository interface {
	Exec(callback func(db *sql.DB) (error)) (error)
	Query(callback func(db *sql.DB) (interface{}, error)) (interface{}, error)
}

type baseRepository struct {
	contract BaseContract
}

// Constructor
func NewBaseRepo(contract BaseContract) *baseRepository {
	return &baseRepository{
		contract,
	}
}

// Wrapper method for func(db *sql.DB) (error) callbacks
func (b *baseRepository) Exec(callback func(db *sql.DB) (error)) (error) {
	db, err := GetDb()
	if err != nil {
		return err
	}

	conn, err := db.GetConnection()
	if err != nil {
		return err
	}
	
	return callback(conn)
}

// Wrapper method for func(db *sql.DB) (interface{}, error) callbacks
func (b *baseRepository) Query(callback func(db *sql.DB) (interface{}, error)) (interface{}, error) {
	db, err := GetDb()
	if err != nil {
		return false, err
	}

	conn, err := db.GetConnection()
	if err != nil {
		return false, err
	}

	return callback(conn)
}