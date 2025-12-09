// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/logging"
	"storj.io/storj/private/migrate"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/admin/back-office/changehistory"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/bucketmigrations"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/nodeapiversion"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/satellite/revocation"
	"storj.io/storj/satellite/satellitedb/consoledb"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/satellite/snopayouts"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/flightrecorder"
	"storj.io/storj/shared/lrucache"
	"storj.io/storj/shared/tagsql"
)

// Error is the default satellitedb errs class.
var Error = errs.Class("satellitedb")

type satelliteDBCollection struct {
	dbs            map[string]*satelliteDB
	maxCommitDelay *time.Duration
}

// satelliteDB combines access to different database tables with a record
// of the db driver, db implementation, and db source URL.
type satelliteDB struct {
	*dbx.DB

	migrationDB tagsql.DB

	opts   Options
	log    *zap.Logger
	driver string
	impl   dbutil.Implementation
	source string

	consoleDBOnce sync.Once
	consoleDB     *consoledb.ConsoleDB

	revocationDBOnce sync.Once
	revocationDB     *revocationDB
}

// Options includes options for how a satelliteDB runs.
type Options struct {
	ApplicationName      string
	APIKeysLRUOptions    lrucache.Options
	RevocationLRUOptions lrucache.Options

	// How many storage node rollups to save/read in one batch.
	SaveRollupBatchSize int
	ReadRollupBatchSize int

	FlightRecorder *flightrecorder.Box

	MaxCommitDelay *time.Duration
}

var _ dbx.DBMethods = &satelliteDB{}

var safelyPartitionableDBs = map[string]bool{
	// WARNING: only list additional db names here after they have been
	// validated to be safely partitionable and that they do not do
	// cross-db queries.
	"repairqueue":   true,
	"nodeevents":    true,
	"verifyqueue":   true,
	"reverifyqueue": true,
	"overlaycache":  true, // tables: nodes, node_tags
}

// Open creates instance of satellite.DB.
func Open(ctx context.Context, log *zap.Logger, databaseURL string, opts Options) (rv satellite.DB, err error) {
	dbMapping, err := dbutil.ParseDBMapping(databaseURL)
	if err != nil {
		return nil, err
	}

	dbc := &satelliteDBCollection{
		dbs:            map[string]*satelliteDB{},
		maxCommitDelay: opts.MaxCommitDelay,
	}
	defer func() {
		if err != nil {
			err = errs.Combine(err, dbc.Close())
		}
	}()

	for key, val := range dbMapping {
		db, openErr := open(ctx, log, val, opts, key)
		if openErr != nil {
			err = errs.Combine(err, openErr)
			return nil, err
		}
		dbc.dbs[key] = db
	}

	return dbc, nil
}

func open(ctx context.Context, log *zap.Logger, databaseURL string, opts Options, override string) (*satelliteDB, error) {
	driver, source, impl, err := dbutil.SplitConnStr(databaseURL)
	if err != nil {
		return nil, err
	}
	if impl != dbutil.Postgres && impl != dbutil.Cockroach && impl != dbutil.Spanner {
		return nil, Error.New("unsupported driver %q", driver)
	}

	// spanner does not have an application name option in the connection string
	if impl == dbutil.Postgres || impl == dbutil.Cockroach {
		source, err = pgutil.EnsureApplicationName(source, opts.ApplicationName)
		if err != nil {
			return nil, err
		}
	}

	dbxSource := source
	if impl == dbutil.Spanner {
		params, err := spannerutil.ParseConnStr(source)
		if err != nil {
			return nil, Error.New("invalid connection string for Spanner: %w", err)
		}
		params.UserAgent = opts.ApplicationName
		dbxSource = params.GoSqlSpannerConnStr()
	}

	dbxDB, err := dbx.Open(driver, dbxSource, opts.FlightRecorder)
	if err != nil {
		return nil, Error.New("failed opening database via DBX at %q: %v", dbxSource, err)
	}

	if log.Level() == zap.DebugLevel {
		log.Debug("Connected to:", zap.String("db source", logging.Redacted(source)))
	}

	name := "satellitedb"
	if override != "" {
		name += ":" + override
	}
	dbutil.Configure(ctx, dbxDB.DB, name, mon)

	core := &satelliteDB{
		DB: dbxDB,

		opts:   opts,
		log:    log,
		driver: driver,
		impl:   impl,
		source: source,
	}

	core.migrationDB = core

	return core, nil
}

func (dbc *satelliteDBCollection) getByName(name string) *satelliteDB {
	if safelyPartitionableDBs[name] {
		if db, exists := dbc.dbs[name]; exists {
			return db
		}
	}
	return dbc.dbs[""]
}

// PeerIdentities returns a storage for peer identities.
func (dbc *satelliteDBCollection) PeerIdentities() overlay.PeerIdentities {
	return &peerIdentities{db: dbc.getByName("peeridentities")}
}

// Attribution is a getter for value attribution repository.
func (dbc *satelliteDBCollection) Attribution() attribution.DB {
	return &attributionDB{db: dbc.getByName("attribution")}
}

// OverlayCache is a getter for overlay cache repository.
func (dbc *satelliteDBCollection) OverlayCache() overlay.DB {
	return &overlaycache{db: dbc.getByName("overlaycache")}
}

// NodeEvents is a getter for node events repository.
func (dbc *satelliteDBCollection) NodeEvents() nodeevents.DB {
	return &nodeEvents{db: dbc.getByName("nodeevents")}
}

// Reputation is a getter for overlay cache repository.
func (dbc *satelliteDBCollection) Reputation() reputation.DB {
	return &reputations{db: dbc.getByName("reputations")}
}

// RepairQueue is a getter for RepairQueue repository.
func (dbc *satelliteDBCollection) RepairQueue() queue.RepairQueue {
	return &repairQueue{db: dbc.getByName("repairqueue")}
}

// VerifyQueue is a getter for VerifyQueue database.
func (dbc *satelliteDBCollection) VerifyQueue() audit.VerifyQueue {
	return &verifyQueue{db: dbc.getByName("verifyqueue")}
}

// ReverifyQueue is a getter for ReverifyQueue database.
func (dbc *satelliteDBCollection) ReverifyQueue() audit.ReverifyQueue {
	return &reverifyQueue{db: dbc.getByName("reverifyqueue")}
}

// StoragenodeAccounting returns database for tracking storagenode usage.
func (dbc *satelliteDBCollection) StoragenodeAccounting() accounting.StoragenodeAccounting {
	return &StoragenodeAccounting{db: dbc.getByName("storagenodeaccounting")}
}

// ProjectAccounting returns database for tracking project data use.
func (dbc *satelliteDBCollection) ProjectAccounting() accounting.ProjectAccounting {
	return &ProjectAccounting{db: dbc.getByName("projectaccounting")}
}

// Revocation returns the database to deal with macaroon revocation.
func (dbc *satelliteDBCollection) Revocation() revocation.DB {
	db := dbc.getByName("revocation")
	db.revocationDBOnce.Do(func() {
		options := db.opts.RevocationLRUOptions
		options.Name = "satellitedb-revocations"
		db.revocationDB = &revocationDB{
			db:      db,
			lru:     lrucache.NewOf[bool](options),
			methods: db,
		}
	})
	return db.revocationDB
}

// Console returns database for storing users, projects and api keys.
func (dbc *satelliteDBCollection) Console() console.DB {
	db := dbc.getByName("console")
	db.consoleDBOnce.Do(func() {
		db.consoleDB = &consoledb.ConsoleDB{
			DB:                db.DB,
			ApikeysLRUOptions: db.opts.APIKeysLRUOptions,

			Impl:    db.impl,
			Methods: db,

			ApikeysOnce: new(sync.Once),
		}
	})

	return db.consoleDB
}

// AdminChangeHistory returns the database for storing admin change history.
func (dbc *satelliteDBCollection) AdminChangeHistory() changehistory.DB {
	return &ChangeHistories{db: dbc.getByName("adminchangehistory")}
}

// OIDC returns the database for storing OAuth and OIDC information.
func (dbc *satelliteDBCollection) OIDC() oidc.DB {
	db := dbc.getByName("oidc")
	return oidc.NewDB(db.DB)
}

// Orders returns database for storing orders.
func (dbc *satelliteDBCollection) Orders() orders.DB {
	db := dbc.getByName("orders")
	return &ordersDB{
		db:             db,
		maxCommitDelay: dbc.maxCommitDelay,
	}
}

// Containment returns database for storing pending audit info.
// It does all of its work by way of the ReverifyQueue.
func (dbc *satelliteDBCollection) Containment() audit.Containment {
	return &containment{reverifyQueue: dbc.ReverifyQueue()}
}

// StripeCoinPayments returns database for stripecoinpayments.
func (dbc *satelliteDBCollection) StripeCoinPayments() stripe.DB {
	return &stripeCoinPaymentsDB{db: dbc.getByName("stripecoinpayments")}
}

// Billing returns database for billing and payment transactions.
func (dbc *satelliteDBCollection) Billing() billing.TransactionsDB {
	return &billingDB{db: dbc.getByName("billing")}
}

// Wallets returns database for storjscan wallets.
func (dbc *satelliteDBCollection) Wallets() storjscan.WalletsDB {
	return &storjscanWalletsDB{db: dbc.getByName("storjscan")}
}

// SNOPayouts returns database for storagenode payStubs and payments info.
func (dbc *satelliteDBCollection) SNOPayouts() snopayouts.DB {
	return &snopayoutsDB{db: dbc.getByName("snopayouts")}
}

// Compensation returns database for storage node compensation.
func (dbc *satelliteDBCollection) Compensation() compensation.DB {
	return &compensationDB{db: dbc.getByName("compensation")}
}

// NodeAPIVersion returns database for storage node api version lower bounds.
func (dbc *satelliteDBCollection) NodeAPIVersion() nodeapiversion.DB {
	return &nodeAPIVersionDB{db: dbc.getByName("nodeapiversion")}
}

// Buckets returns database for interacting with buckets.
func (dbc *satelliteDBCollection) Buckets() buckets.DB {
	return &bucketsDB{db: dbc.getByName("buckets")}
}

// BucketMigrations returns database for interacting with bucket migrations.
func (dbc *satelliteDBCollection) BucketMigrations() bucketmigrations.DB {
	return &bucketMigrationsDB{db: dbc.getByName("bucketmigrations")}
}

// StorjscanPayments returns database for storjscan payments.
func (dbc *satelliteDBCollection) StorjscanPayments() storjscan.PaymentsDB {
	return &storjscanPayments{db: dbc.getByName("storjscan_payments")}
}

// CheckVersion confirms all databases are at the desired version.
func (dbc *satelliteDBCollection) CheckVersion(ctx context.Context) error {
	var eg errs.Group
	for _, db := range dbc.dbs {
		eg.Add(db.CheckVersion(ctx))
	}
	return eg.Err()
}

// MigrateToLatest migrates all databases to the latest version.
func (dbc *satelliteDBCollection) MigrateToLatest(ctx context.Context) error {
	var eg errs.Group
	for _, db := range dbc.dbs {
		eg.Add(db.MigrateToLatest(ctx))
	}
	return eg.Err()
}

// Close closes all satellite dbs.
func (dbc *satelliteDBCollection) Close() error {
	var eg errs.Group
	for _, db := range dbc.dbs {
		eg.Add(db.Close())
	}
	return eg.Err()
}

// Testing provides access to testing facilities. These should not be used in production code.
func (db *satelliteDB) Testing() satellite.TestingDB {
	return &satelliteDBTesting{satelliteDB: db}
}

type satelliteDBTesting struct{ *satelliteDB }

// Implementation returns the implementations of the databases.
func (db *satelliteDBTesting) Implementation() []dbutil.Implementation {
	return []dbutil.Implementation{db.satelliteDB.impl}
}

// Rebind adapts a query's syntax for a database dialect.
func (db *satelliteDBTesting) Rebind(query string) string {
	return db.satelliteDB.Rebind(query)
}

// RawDB returns the underlying database connection to the primary database.
func (db *satelliteDBTesting) RawDB() tagsql.DB {
	return db.satelliteDB.DB
}

// Schema returns the full schema for the database.
func (db *satelliteDBTesting) Schema() []string {
	return db.satelliteDB.Schema()
}

// ProductionMigration returns the primary migration.
func (db *satelliteDBTesting) ProductionMigration() *migrate.Migration {
	return db.satelliteDB.ProductionMigration()
}

// TestMigration returns the migration used for tests.
func (db *satelliteDBTesting) TestMigration() *migrate.Migration {
	return db.satelliteDB.TestMigration()
}

// Testing provides access to testing facilities. These should not be used in production code.
func (dbc *satelliteDBCollection) Testing() satellite.TestingDB {
	return &satelliteDBCollectionTesting{satelliteDBCollection: dbc}
}

type satelliteDBCollectionTesting struct{ *satelliteDBCollection }

// Implementation returns the implementations of the databases.
func (dbc *satelliteDBCollectionTesting) Implementation() []dbutil.Implementation {
	var r []dbutil.Implementation
	for _, db := range dbc.dbs {
		r = append(r, db.impl)
	}
	return r
}

// Rebind adapts a query's syntax for a database dialect.
func (dbc *satelliteDBCollectionTesting) Rebind(query string) string {
	return dbc.getByName("").Rebind(query)
}

// RawDB returns the underlying database connection to the primary database.
func (dbc *satelliteDBCollectionTesting) RawDB() tagsql.DB {
	return dbc.getByName("").DB.DB
}

// Schema returns the full schema for the database.
func (dbc *satelliteDBCollectionTesting) Schema() []string {
	return dbc.getByName("").Schema()
}

// MigrateToLatest initializes the database for testplanet.
func (dbc *satelliteDBCollectionTesting) TestMigrateToLatest(ctx context.Context) error {
	var eg errs.Group
	for _, db := range dbc.dbs {
		eg.Add(db.Testing().TestMigrateToLatest(ctx))
	}
	return eg.Err()
}

// ProductionMigration returns the primary migration.
func (dbc *satelliteDBCollectionTesting) ProductionMigration() *migrate.Migration {
	return dbc.getByName("").ProductionMigration()
}

// TestMigration returns the migration used for tests.
func (dbc *satelliteDBCollectionTesting) TestMigration() *migrate.Migration {
	return dbc.getByName("").TestMigration()
}
