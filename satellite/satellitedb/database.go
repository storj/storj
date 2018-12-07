// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"github.com/zeebo/errs"

	"storj.io/storj/internal/migrate"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

var (
	// Error is the default satellitedb errs class
	Error = errs.Class("satellitedb")
)

// DB contains access to different database tables
type DB struct {
	db *dbx.DB
}

// NewDB creates instance of database (supports: postgres, sqlite3)
func NewDB(databaseURL string) (*DB, error) {
	dbURL, err := utils.ParseURL(databaseURL)
	if err != nil {
		return nil, err
	}
	source := databaseURL
	if dbURL.Scheme == "sqlite3" {
		source = dbURL.Path
	}
	db, err := dbx.Open(dbURL.Scheme, source)
	if err != nil {
		return nil, Error.New("failed opening database %q, %q: %v",
			dbURL.Scheme, source, err)
	}

	return &DB{db: db}, nil
}

// BandwidthAgreement is a getter for bandwidth agreement repository
func (db *DB) BandwidthAgreement() bwagreement.DB {
	return &bandwidthagreement{db: db.db}
}

// // PointerDB is a getter for PointerDB repository
// func (db *DB) PointerDB() pointerdb.DB {
// 	return &pointerDB{db: db.db}
// }

// // StatDB is a getter for StatDB repository
// func (db *DB) StatDB() statdb.DB {
// 	return &statDB{db: db.db}
// }

// // OverlayCacheDB is a getter for OverlayCacheDB repository
// func (db *DB) OverlayCacheDB() overlay.DB {
// 	return &overlayCacheDB{db: db.db}
// }

// // RepairQueueDB is a getter for RepairQueueDB repository
// func (db *DB) RepairQueueDB() queue.DB {
// 	return &repairQueueDB{db: db.db}
// }

// // AccountingDB is a getter for AccountingDB repository
// func (db *DB) AccountingDB() accounting.DB {
// 	return &accountingDB{db: db.db}
// }

// CreateTables is a method for creating all tables for database
func (db *DB) CreateTables() error {
	return migrate.Create("database", db.db)
}

// Close is used to close db connection
func (db *DB) Close() error {
	return db.db.Close()
}
