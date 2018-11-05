// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accountdb

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
)

// Data base connection factory
type dbFactory struct {
	conn *sql.DB
	constr string
	driver string
}

// strings, messages, etc
var (
	notInitializedErrorMsg = "DbFactory is not initialized!.\nUse NewDbFactory function to initialize it."
)

// db instance.
var db *dbFactory = nil

// NewDbFactory initializes new dbFactory instance and returns pointer on it.
func NewDbFactory(connstr, driver string) *dbFactory {

	// is it ok? 0_o
	if db != nil {
		return db
	}

	db = &dbFactory {
		nil,
		connstr,
		driver,
	}

	return db
}

// GetDb returns pointer on dbFactory instance.
// NewDbFactory should be called before.
func GetDb() (*dbFactory, error) {

	if db == nil {
		return nil, errors.New(notInitializedErrorMsg)
	}

	return db, nil
}

// Establish connection with db
func (db *dbFactory) GetConnection() (*sql.DB, error) {

	if db.conn == nil {

		conn, err := sql.Open(db.driver, db.constr)

		if err != nil {
			return nil, err
		}

		db.conn = conn
	}

	return db.conn, nil
}
