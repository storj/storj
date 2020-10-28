// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/cache"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/tagsql"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/downtime"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/nodeapiversion"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/repair/irreparable"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/revocation"
	"storj.io/storj/satellite/rewards"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/satellite/snopayout"
)

var (
	// Error is the default satellitedb errs class.
	Error = errs.Class("satellitedb")
)

// satelliteDB combines access to different database tables with a record
// of the db driver, db implementation, and db source URL.
type satelliteDB struct {
	*dbx.DB

	migrationDB tagsql.DB

	opts           Options
	log            *zap.Logger
	driver         string
	implementation dbutil.Implementation
	source         string

	consoleDBOnce sync.Once
	consoleDB     *ConsoleDB

	revocationDBOnce sync.Once
	revocationDB     *revocationDB
}

// Options includes options for how a satelliteDB runs.
type Options struct {
	APIKeysLRUOptions    cache.Options
	RevocationLRUOptions cache.Options

	// How many records to read in a single transaction when asked for all of the
	// billable bandwidth from the reported serials table.
	ReportedRollupsReadBatchSize int
}

var _ dbx.DBMethods = &satelliteDB{}

// Open creates instance of database supports postgres.
func Open(ctx context.Context, log *zap.Logger, databaseURL string, opts Options) (satellite.DB, error) {
	driver, source, implementation, err := dbutil.SplitConnStr(databaseURL)
	if err != nil {
		return nil, err
	}
	if implementation != dbutil.Postgres && implementation != dbutil.Cockroach {
		return nil, Error.New("unsupported driver %q", driver)
	}

	source = pgutil.CheckApplicationName(source)

	dbxDB, err := dbx.Open(driver, source)
	if err != nil {
		return nil, Error.New("failed opening database via DBX at %q: %v",
			source, err)
	}
	log.Debug("Connected to:", zap.String("db source", source))

	dbutil.Configure(dbxDB.DB, "satellitedb", mon)

	core := &satelliteDB{
		DB: dbxDB,

		opts:           opts,
		log:            log,
		driver:         driver,
		implementation: implementation,
		source:         source,
	}

	core.migrationDB = core

	return core, nil
}

// TestDBAccess for raw database access,
// should not be used outside of migration tests.
func (db *satelliteDB) TestDBAccess() *dbx.DB { return db.DB }

// PeerIdentities returns a storage for peer identities.
func (db *satelliteDB) PeerIdentities() overlay.PeerIdentities {
	return &peerIdentities{db: db}
}

// Attribution is a getter for value attribution repository.
func (db *satelliteDB) Attribution() attribution.DB {
	return &attributionDB{db: db}
}

// OverlayCache is a getter for overlay cache repository.
func (db *satelliteDB) OverlayCache() overlay.DB {
	return &overlaycache{db: db}
}

// RepairQueue is a getter for RepairQueue repository.
func (db *satelliteDB) RepairQueue() queue.RepairQueue {
	return &repairQueue{db: db}
}

// StoragenodeAccounting returns database for tracking storagenode usage.
func (db *satelliteDB) StoragenodeAccounting() accounting.StoragenodeAccounting {
	return &StoragenodeAccounting{db: db}
}

// ProjectAccounting returns database for tracking project data use.
func (db *satelliteDB) ProjectAccounting() accounting.ProjectAccounting {
	return &ProjectAccounting{db: db}
}

// Irreparable returns database for storing segments that failed repair.
func (db *satelliteDB) Irreparable() irreparable.DB {
	return &irreparableDB{db: db}
}

// Revocation returns the database to deal with macaroon revocation.
func (db *satelliteDB) Revocation() revocation.DB {
	db.revocationDBOnce.Do(func() {
		db.revocationDB = &revocationDB{
			db:      db,
			lru:     cache.New(db.opts.RevocationLRUOptions),
			methods: db,
		}
	})
	return db.revocationDB
}

// Console returns database for storing users, projects and api keys.
func (db *satelliteDB) Console() console.DB {
	db.consoleDBOnce.Do(func() {
		db.consoleDB = &ConsoleDB{
			apikeysLRUOptions: db.opts.APIKeysLRUOptions,

			db:      db,
			methods: db,

			apikeysOnce: new(sync.Once),
		}
	})

	return db.consoleDB
}

// Rewards returns database for storing offers.
func (db *satelliteDB) Rewards() rewards.DB {
	return &offersDB{db: db}
}

// Orders returns database for storing orders.
func (db *satelliteDB) Orders() orders.DB {
	return &ordersDB{db: db, reportedRollupsReadBatchSize: db.opts.ReportedRollupsReadBatchSize}
}

// Containment returns database for storing pending audit info.
func (db *satelliteDB) Containment() audit.Containment {
	return &containment{db: db}
}

// GracefulExit returns database for graceful exit.
func (db *satelliteDB) GracefulExit() gracefulexit.DB {
	return &gracefulexitDB{db: db}
}

// StripeCoinPayments returns database for stripecoinpayments.
func (db *satelliteDB) StripeCoinPayments() stripecoinpayments.DB {
	return &stripeCoinPaymentsDB{db: db}
}

// DowntimeTracking returns database for downtime tracking.
func (db *satelliteDB) DowntimeTracking() downtime.DB {
	return &downtimeTrackingDB{db: db}
}

// SnoPayout returns database for storagenode payStubs and payments info.
func (db *satelliteDB) SnoPayout() snopayout.DB {
	return &paymentStubs{db: db}
}

// Compenstation returns database for storage node compensation.
func (db *satelliteDB) Compensation() compensation.DB {
	return &compensationDB{db: db}
}

// NodeAPIVersion returns database for storage node api version lower bounds.
func (db *satelliteDB) NodeAPIVersion() nodeapiversion.DB {
	return &nodeAPIVersionDB{db: db}
}
