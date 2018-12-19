// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"github.com/zeebo/errs"

	"storj.io/storj/internal/migrate"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/storage"
)

var (
	// Error is the default satellitedb errs class
	Error = errs.Class("satellitedb")
)

// DB contains access to different database tables
type DB struct {
	db *dbx.DB
}

// New creates instance of database (supports: postgres, sqlite3)
func New(databaseURL string) (*DB, error) {
	driver, source, err := utils.SplitDBURL(databaseURL)
	if err != nil {
		return nil, err
	}
	db, err := dbx.Open(driver, source)
	if err != nil {
		return nil, Error.New("failed opening database %q, %q: %v",
			driver, source, err)
	}
	return &DB{db: db}, nil
}

// NewInMemory creates instance of Sqlite in memory satellite database
func NewInMemory() (*DB, error) {
	return New("sqlite3://file::memory:?mode=memory&cache=shared")
}

// BandwidthAgreement is a getter for bandwidth agreement repository
func (db *DB) BandwidthAgreement() bwagreement.DB {
	return &bandwidthagreement{db: db.db}
}

// // PointerDB is a getter for PointerDB repository
// func (db *DB) PointerDB() pointerdb.DB {
// 	return &pointerDB{db: db.db}
// }

// StatDB is a getter for StatDB repository
func (db *DB) StatDB() statdb.DB {
	return &statDB{db: db.db}
}

// OverlayCache is a getter for overlay cache repository
func (db *DB) OverlayCache() storage.KeyValueStore {
	return newOverlaycache(db.db)
}

// RepairQueue is a getter for RepairQueue repository
func (db *DB) RepairQueue() queue.RepairQueue {
	return newRepairQueue(db.db)
}

// Accounting returns database for tracking bandwidth agreements over time
func (db *DB) Accounting() accounting.DB {
	return &accountingDB{db: db.db}
}

// Irreparable returns database for storing segments that failed repair
func (db *DB) Irreparable() irreparable.DB {
	return &irreparableDB{db: db.db}
}

// CreateTables is a method for creating all tables for database
func (db *DB) CreateTables() error {
	return migrate.Create("database", db.db)
}

// Close is used to close db connection
func (db *DB) Close() error {
	return db.db.Close()
}
