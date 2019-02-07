// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"strconv"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/migrate"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/certdb"
	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

var (
	// Error is the default satellitedb errs class
	Error = errs.Class("satellitedb")
)

//go:generate go run ../../scripts/lockedgen.go -o locked.go -p satellitedb -i storj.io/storj/satellite.DB

// DB contains access to different database tables
type DB struct {
	db     *dbx.DB
	driver string
}

// New creates instance of database (supports: postgres, sqlite3)
func New(databaseURL string) (satellite.DB, error) {
	driver, source, err := utils.SplitDBURL(databaseURL)
	if err != nil {
		return nil, err
	}

	db, err := dbx.Open(driver, source)
	if err != nil {
		return nil, Error.New("failed opening database %q, %q: %v",
			driver, source, err)
	}

	core := &DB{db: db, driver: driver}
	if driver == "sqlite3" {
		return newLocked(core), nil
	}
	return core, nil
}

// NewInMemory creates instance of Sqlite in memory satellite database
func NewInMemory() (satellite.DB, error) {
	return New("sqlite3://file::memory:?mode=memory")
}

// CreateSchema creates a schema if it doesn't exist.
func (db *DB) CreateSchema(schema string) error {
	switch db.driver {
	case "postgres":
		_, err := db.db.Exec(`create schema if not exists ` + quoteSchema(schema) + `;`)
		return err
	}
	return nil
}

// DropSchema drops the named schema
func (db *DB) DropSchema(schema string) error {
	switch db.driver {
	case "postgres":
		_, err := db.db.Exec(`drop schema ` + quoteSchema(schema) + ` cascade;`)
		return err
	}
	return nil
}

// quoteSchema quotes schema name such that it can be used in a postgres query
func quoteSchema(schema string) string {
	return strconv.QuoteToASCII(schema)
}

// BandwidthAgreement is a getter for bandwidth agreement repository
func (db *DB) BandwidthAgreement() bwagreement.DB {
	return &bandwidthagreement{db: db.db}
}

// CertDB is a getter for uplink's specific info like public key, id, etc...
func (db *DB) CertDB() certdb.DB {
	return &certDB{db: db.db}
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
func (db *DB) OverlayCache() overlay.DB {
	return &overlaycache{db: db.db}
}

// RepairQueue is a getter for RepairQueue repository
func (db *DB) RepairQueue() queue.RepairQueue {
	return &repairQueue{db: db.db}
}

// Accounting returns database for tracking bandwidth agreements over time
func (db *DB) Accounting() accounting.DB {
	return &accountingDB{db: db.db}
}

// Irreparable returns database for storing segments that failed repair
func (db *DB) Irreparable() irreparable.DB {
	return &irreparableDB{db: db.db}
}

// Console returns database for storing users, projects and api keys
func (db *DB) Console() console.DB {
	return &ConsoleDB{
		db:      db.db,
		methods: db.db,
	}
}

// CreateTables is a method for creating all tables for database
func (db *DB) CreateTables() error {
	return migrate.Create("database", db.db)
}

// Close is used to close db connection
func (db *DB) Close() error {
	return db.db.Close()
}
