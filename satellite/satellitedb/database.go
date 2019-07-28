// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/dbutil"
	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/certdb"
	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/rewards"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

var (
	// Error is the default satellitedb errs class
	Error = errs.Class("satellitedb")
)

//go:generate go run ../../scripts/lockedgen.go -o locked.go -p satellitedb -i storj.io/storj/satellite.DB

// DB contains access to different database tables
type DB struct {
	log    *zap.Logger
	db     *dbx.DB
	driver string
	source string
}

// New creates instance of database (supports: postgres, sqlite3)
func New(log *zap.Logger, databaseURL string) (satellite.DB, error) {
	driver, source, err := dbutil.SplitConnstr(databaseURL)
	if err != nil {
		return nil, err
	}
	if driver == "postgres" {
		source = pgutil.CheckApplicationName(source)
	}
	db, err := dbx.Open(driver, source)
	if err != nil {
		return nil, Error.New("failed opening database %q, %q: %v",
			driver, source, err)
	}
	log.Debug("Connected to:", zap.String("db source", source))

	dbutil.Configure(db.DB, mon)

	core := &DB{log: log, db: db, driver: driver, source: source}
	if driver == "sqlite3" {
		return newLocked(core), nil
	}
	return core, nil
}

// NewInMemory creates instance of Sqlite in memory satellite database
func NewInMemory(log *zap.Logger) (satellite.DB, error) {
	return New(log, "sqlite3://file::memory:?mode=memory")
}

// Close is used to close db connection
func (db *DB) Close() error {
	return db.db.Close()
}

// CreateSchema creates a schema if it doesn't exist.
func (db *DB) CreateSchema(schema string) error {
	if db.driver == "postgres" {
		return pgutil.CreateSchema(db.db, schema)
	}
	return nil
}

// TestDBAccess for raw database access,
// should not be used outside of migration tests.
func (db *DB) TestDBAccess() *dbx.DB { return db.db }

// TestDBAccess for raw database access,
// should not be used outside of tests.
func (db *locked) TestDBAccess() *dbx.DB {
	return db.db.(interface{ TestDBAccess() *dbx.DB }).TestDBAccess()
}

// DropSchema drops the named schema
func (db *DB) DropSchema(schema string) error {
	if db.driver == "postgres" {
		return pgutil.DropSchema(db.db, schema)
	}
	return nil
}

// CertDB is a getter for uplink's specific info like public key, id, etc...
func (db *DB) CertDB() certdb.DB {
	return &certDB{db: db.db}
}

// Attribution is a getter for value attribution repository
func (db *DB) Attribution() attribution.DB {
	return &attributionDB{db: db.db}
}

// OverlayCache is a getter for overlay cache repository
func (db *DB) OverlayCache() overlay.DB {
	return &overlaycache{db: db.db}
}

// RepairQueue is a getter for RepairQueue repository
func (db *DB) RepairQueue() queue.RepairQueue {
	return &repairQueue{db: db.db}
}

// StoragenodeAccounting returns database for tracking storagenode usage
func (db *DB) StoragenodeAccounting() accounting.StoragenodeAccounting {
	return &StoragenodeAccounting{db: db.db}
}

// ProjectAccounting returns database for tracking project data use
func (db *DB) ProjectAccounting() accounting.ProjectAccounting {
	return &ProjectAccounting{db: db.db}
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

// Rewards returns database for storing offers
func (db *DB) Rewards() rewards.DB {
	return &offersDB{db: db.db}
}

// Orders returns database for storing orders
func (db *DB) Orders() orders.DB {
	return &ordersDB{db: db.db}
}

// Containment returns database for storing pending audit info
func (db *DB) Containment() audit.Containment {
	return &containment{db: db.db}
}
