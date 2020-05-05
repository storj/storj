// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:generate sh -c "go run schemagen.go > schema.go.tmp && mv schema.go.tmp schema.go"

package storagenodedb

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-cmp/cmp"
	_ "github.com/mattn/go-sqlite3" // used indirectly.
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/sqliteutil"
	"storj.io/storj/private/migrate"
	"storj.io/storj/private/tagsql"
	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/heldamount"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/storageusage"
)

// VersionTable is the table that stores the version info in each db
const VersionTable = "versions"

var (
	mon = monkit.Package()

	// ErrDatabase represents errors from the databases.
	ErrDatabase = errs.Class("storage node database error")
	// ErrNoRows represents database error if rows weren't affected.
	ErrNoRows = errs.New("no rows affected")
	// ErrPreflight represents an error during the preflight check.
	ErrPreflight = errs.Class("storage node preflight database error")
)

// DBContainer defines an interface to allow accessing and setting a SQLDB
type DBContainer interface {
	Configure(sqlDB tagsql.DB)
	GetDB() tagsql.DB
}

// withTx is a helper method which executes callback in transaction scope
func withTx(ctx context.Context, db tagsql.DB, cb func(tx tagsql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errs.Combine(err, tx.Rollback())
			return
		}

		err = tx.Commit()
	}()
	return cb(tx)
}

// Config configures storage node database
type Config struct {
	// TODO: figure out better names
	Storage   string
	Info      string
	Info2     string
	Driver    string // if unset, uses sqlite3
	Pieces    string
	Filestore filestore.Config
}

// DB contains access to different database tables
type DB struct {
	log    *zap.Logger
	config Config

	pieces storage.Blobs

	dbDirectory string

	deprecatedInfoDB  *deprecatedInfoDB
	v0PieceInfoDB     *v0PieceInfoDB
	bandwidthDB       *bandwidthDB
	ordersDB          *ordersDB
	pieceExpirationDB *pieceExpirationDB
	pieceSpaceUsedDB  *pieceSpaceUsedDB
	reputationDB      *reputationDB
	storageUsageDB    *storageUsageDB
	usedSerialsDB     *usedSerialsDB
	satellitesDB      *satellitesDB
	notificationsDB   *notificationDB
	heldamountDB      *heldamountDB
	pricingDB         *pricingDB

	SQLDBs map[string]DBContainer
}

// New creates a new master database for storage node
func New(log *zap.Logger, config Config) (*DB, error) {
	piecesDir, err := filestore.NewDir(config.Pieces)
	if err != nil {
		return nil, err
	}
	pieces := filestore.New(log, piecesDir, config.Filestore)

	deprecatedInfoDB := &deprecatedInfoDB{}
	v0PieceInfoDB := &v0PieceInfoDB{}
	bandwidthDB := &bandwidthDB{}
	ordersDB := &ordersDB{}
	pieceExpirationDB := &pieceExpirationDB{}
	pieceSpaceUsedDB := &pieceSpaceUsedDB{}
	reputationDB := &reputationDB{}
	storageUsageDB := &storageUsageDB{}
	usedSerialsDB := &usedSerialsDB{}
	satellitesDB := &satellitesDB{}
	notificationsDB := &notificationDB{}
	heldamountDB := &heldamountDB{}
	pricingDB := &pricingDB{}

	db := &DB{
		log:    log,
		config: config,

		pieces: pieces,

		dbDirectory: filepath.Dir(config.Info2),

		deprecatedInfoDB:  deprecatedInfoDB,
		v0PieceInfoDB:     v0PieceInfoDB,
		bandwidthDB:       bandwidthDB,
		ordersDB:          ordersDB,
		pieceExpirationDB: pieceExpirationDB,
		pieceSpaceUsedDB:  pieceSpaceUsedDB,
		reputationDB:      reputationDB,
		storageUsageDB:    storageUsageDB,
		usedSerialsDB:     usedSerialsDB,
		satellitesDB:      satellitesDB,
		notificationsDB:   notificationsDB,
		heldamountDB:      heldamountDB,
		pricingDB:         pricingDB,

		SQLDBs: map[string]DBContainer{
			DeprecatedInfoDBName:  deprecatedInfoDB,
			PieceInfoDBName:       v0PieceInfoDB,
			BandwidthDBName:       bandwidthDB,
			OrdersDBName:          ordersDB,
			PieceExpirationDBName: pieceExpirationDB,
			PieceSpaceUsedDBName:  pieceSpaceUsedDB,
			ReputationDBName:      reputationDB,
			StorageUsageDBName:    storageUsageDB,
			UsedSerialsDBName:     usedSerialsDB,
			SatellitesDBName:      satellitesDB,
			NotificationsDBName:   notificationsDB,
			HeldAmountDBName:      heldamountDB,
			PricingDBName:         pricingDB,
		},
	}

	err = db.openDatabases()
	if err != nil {
		return nil, err
	}
	return db, nil
}

// openDatabases opens all the SQLite3 storage node databases and returns if any fails to open successfully.
func (db *DB) openDatabases() error {
	// These objects have a Configure method to allow setting the underlining SQLDB connection
	// that each uses internally to do data access to the SQLite3 databases.
	// The reason it was done this way was because there's some outside consumers that are
	// taking a reference to the business object.
	err := db.openDatabase(DeprecatedInfoDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}

	err = db.openDatabase(BandwidthDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}

	err = db.openDatabase(OrdersDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}

	err = db.openDatabase(PieceExpirationDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}

	err = db.openDatabase(PieceInfoDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}

	err = db.openDatabase(PieceSpaceUsedDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}

	err = db.openDatabase(ReputationDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}

	err = db.openDatabase(StorageUsageDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}

	err = db.openDatabase(UsedSerialsDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}

	err = db.openDatabase(SatellitesDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}

	err = db.openDatabase(NotificationsDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}

	err = db.openDatabase(HeldAmountDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}

	err = db.openDatabase(PricingDBName)
	if err != nil {
		return errs.Combine(err, db.closeDatabases())
	}
	return nil
}

func (db *DB) rawDatabaseFromName(dbName string) tagsql.DB {
	return db.SQLDBs[dbName].GetDB()
}

// openDatabase opens or creates a database at the specified path.
func (db *DB) openDatabase(dbName string) error {
	path := db.filepathFromDBName(dbName)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return ErrDatabase.Wrap(err)
	}

	driver := db.config.Driver
	if driver == "" {
		driver = "sqlite3"
	}

	sqlDB, err := tagsql.Open(driver, "file:"+path+"?_journal=WAL&_busy_timeout=10000")
	if err != nil {
		return ErrDatabase.Wrap(err)
	}

	mDB := db.SQLDBs[dbName]
	mDB.Configure(sqlDB)

	dbutil.Configure(sqlDB, dbName, mon)

	return nil
}

// filenameFromDBName returns a constructed filename for the specified database name.
func (db *DB) filenameFromDBName(dbName string) string {
	return dbName + ".db"
}

func (db *DB) filepathFromDBName(dbName string) string {
	return filepath.Join(db.dbDirectory, db.filenameFromDBName(dbName))
}

// MigrateToLatest creates any necessary tables.
func (db *DB) MigrateToLatest(ctx context.Context) error {
	migration := db.Migration(ctx)
	return migration.Run(ctx, db.log.Named("migration"))
}

// Preflight conducts a pre-flight check to ensure correct schemas and minimal read+write functionality of the database tables.
func (db *DB) Preflight(ctx context.Context) (err error) {
	for dbName, dbContainer := range db.SQLDBs {
		nextDB := dbContainer.GetDB()
		// Preflight stage 1: test schema correctness
		schema, err := sqliteutil.QuerySchema(ctx, nextDB)
		if err != nil {
			return ErrPreflight.New("%s: schema check failed: %v", dbName, err)
		}
		// we don't care about changes in versions table
		schema.DropTable("versions")
		// if there was a previous pre-flight failure, test_table might still be in the schema
		schema.DropTable("test_table")

		// If tables and indexes of the schema are empty, set to nil
		// to help with comparison to the snapshot.
		if len(schema.Tables) == 0 {
			schema.Tables = nil
		}
		if len(schema.Indexes) == 0 {
			schema.Indexes = nil
		}

		// get expected schema and expect it to match actual schema
		expectedSchema := Schema()[dbName]
		if diff := cmp.Diff(expectedSchema, schema); diff != "" {
			return ErrPreflight.New("%s: expected schema does not match actual: %s", dbName, diff)
		}

		// Preflight stage 2: test basic read/write access
		// for each database, create a new table, insert a row into that table, retrieve and validate that row, and drop the table.

		// drop test table in case the last preflight check failed before table could be dropped
		_, err = nextDB.ExecContext(ctx, "DROP TABLE IF EXISTS test_table")
		if err != nil {
			return ErrPreflight.Wrap(err)
		}
		_, err = nextDB.ExecContext(ctx, "CREATE TABLE test_table(id int NOT NULL, name varchar(30), PRIMARY KEY (id))")
		if err != nil {
			return ErrPreflight.Wrap(err)
		}

		var expectedID, actualID int
		var expectedName, actualName string
		expectedID = 1
		expectedName = "TEST"
		_, err = nextDB.ExecContext(ctx, "INSERT INTO test_table VALUES ( ?, ? )", expectedID, expectedName)
		if err != nil {
			return ErrPreflight.Wrap(err)
		}

		rows, err := nextDB.QueryContext(ctx, "SELECT id, name FROM test_table")
		if err != nil {
			return ErrPreflight.Wrap(err)
		}
		defer func() { err = errs.Combine(err, rows.Close()) }()
		if !rows.Next() {
			return ErrPreflight.New("%s: no rows in test_table", dbName)
		}
		err = rows.Scan(&actualID, &actualName)
		if err != nil {
			return ErrPreflight.Wrap(err)
		}
		if expectedID != actualID || expectedName != actualName {
			return ErrPreflight.New("%s: expected: (%d, '%s'), actual: (%d, '%s')", dbName, expectedID, expectedName, actualID, actualName)
		}
		if rows.Next() {
			return ErrPreflight.New("%s: more than one row in test_table", dbName)
		}

		_, err = nextDB.ExecContext(ctx, "DROP TABLE test_table")
		if err != nil {
			return ErrPreflight.Wrap(err)
		}
	}
	return nil
}

// Close closes any resources.
func (db *DB) Close() error {
	return db.closeDatabases()
}

// closeDatabases closes all the SQLite database connections and removes them from the associated maps.
func (db *DB) closeDatabases() error {
	var errlist errs.Group

	for k := range db.SQLDBs {
		errlist.Add(db.closeDatabase(k))
	}
	return errlist.Err()
}

// closeDatabase closes the specified SQLite database connections and removes them from the associated maps.
func (db *DB) closeDatabase(dbName string) (err error) {
	mdb, ok := db.SQLDBs[dbName]
	if !ok {
		return ErrDatabase.New("no database with name %s found. database was never opened or already closed.", dbName)
	}
	return ErrDatabase.Wrap(mdb.GetDB().Close())
}

// V0PieceInfo returns the instance of the V0PieceInfoDB database.
func (db *DB) V0PieceInfo() pieces.V0PieceInfoDB {
	return db.v0PieceInfoDB
}

// Bandwidth returns the instance of the Bandwidth database.
func (db *DB) Bandwidth() bandwidth.DB {
	return db.bandwidthDB
}

// Orders returns the instance of the Orders database.
func (db *DB) Orders() orders.DB {
	return db.ordersDB
}

// Pieces returns blob storage for pieces
func (db *DB) Pieces() storage.Blobs {
	return db.pieces
}

// PieceExpirationDB returns the instance of the PieceExpiration database.
func (db *DB) PieceExpirationDB() pieces.PieceExpirationDB {
	return db.pieceExpirationDB
}

// PieceSpaceUsedDB returns the instance of the PieceSpacedUsed database.
func (db *DB) PieceSpaceUsedDB() pieces.PieceSpaceUsedDB {
	return db.pieceSpaceUsedDB
}

// Reputation returns the instance of the Reputation database.
func (db *DB) Reputation() reputation.DB {
	return db.reputationDB
}

// StorageUsage returns the instance of the StorageUsage database.
func (db *DB) StorageUsage() storageusage.DB {
	return db.storageUsageDB
}

// UsedSerials returns the instance of the UsedSerials database.
func (db *DB) UsedSerials() piecestore.UsedSerials {
	return db.usedSerialsDB
}

// Satellites returns the instance of the Satellites database.
func (db *DB) Satellites() satellites.DB {
	return db.satellitesDB
}

// Notifications returns the instance of the Notifications database.
func (db *DB) Notifications() notifications.DB {
	return db.notificationsDB
}

// HeldAmount returns instance of the HeldAmount database.
func (db *DB) HeldAmount() heldamount.DB {
	return db.heldamountDB
}

// Pricing returns instance of the Pricing database.
func (db *DB) Pricing() pricing.DB {
	return db.pricingDB
}

// RawDatabases are required for testing purposes
func (db *DB) RawDatabases() map[string]DBContainer {
	return db.SQLDBs
}

// migrateToDB is a helper method that performs the migration from the
// deprecatedInfoDB to the specified new db. It first closes and deletes any
// existing database to guarantee idempotence. After migration it also closes
// and re-opens the new database to allow the system to recover used disk space.
func (db *DB) migrateToDB(ctx context.Context, dbName string, tablesToKeep ...string) error {
	err := db.closeDatabase(dbName)
	if err != nil {
		return ErrDatabase.Wrap(err)
	}

	path := db.filepathFromDBName(dbName)

	if _, err := os.Stat(path); err == nil {
		err = os.Remove(path)
		if err != nil {
			return ErrDatabase.Wrap(err)
		}
	}

	err = db.openDatabase(dbName)
	if err != nil {
		return ErrDatabase.Wrap(err)
	}

	err = sqliteutil.MigrateTablesToDatabase(ctx,
		db.rawDatabaseFromName(DeprecatedInfoDBName),
		db.rawDatabaseFromName(dbName),
		tablesToKeep...)
	if err != nil {
		return ErrDatabase.Wrap(err)
	}

	// We need to close and re-open the database we have just migrated *to* in
	// order to recover any excess disk usage that was freed in the VACUUM call
	err = db.closeDatabase(dbName)
	if err != nil {
		return ErrDatabase.Wrap(err)
	}

	err = db.openDatabase(dbName)
	if err != nil {
		return ErrDatabase.Wrap(err)
	}

	return nil
}

// Migration returns table migrations.
func (db *DB) Migration(ctx context.Context) *migrate.Migration {
	return &migrate.Migration{
		Table: VersionTable,
		Steps: []*migrate.Step{
			{
				DB:          db.deprecatedInfoDB,
				Description: "Initial setup",
				Version:     0,
				Action: migrate.SQL{
					// table for keeping serials that need to be verified against
					`CREATE TABLE used_serial (
						satellite_id  BLOB NOT NULL,
						serial_number BLOB NOT NULL,
						expiration    TIMESTAMP NOT NULL
					)`,
					// primary key on satellite id and serial number
					`CREATE UNIQUE INDEX pk_used_serial ON used_serial(satellite_id, serial_number)`,
					// expiration index to allow fast deletion
					`CREATE INDEX idx_used_serial ON used_serial(expiration)`,

					// certificate table for storing uplink/satellite certificates
					`CREATE TABLE certificate (
						cert_id       INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
						node_id       BLOB        NOT NULL, -- same NodeID can have multiple valid leaf certificates
						peer_identity BLOB UNIQUE NOT NULL  -- PEM encoded
					)`,

					// table for storing piece meta info
					`CREATE TABLE pieceinfo (
						satellite_id     BLOB      NOT NULL,
						piece_id         BLOB      NOT NULL,
						piece_size       BIGINT    NOT NULL,
						piece_expiration TIMESTAMP, -- date when it can be deleted

						uplink_piece_hash BLOB    NOT NULL, -- serialized pb.PieceHash signed by uplink
						uplink_cert_id    INTEGER NOT NULL,

						FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
					)`,
					// primary key by satellite id and piece id
					`CREATE UNIQUE INDEX pk_pieceinfo ON pieceinfo(satellite_id, piece_id)`,

					// table for storing bandwidth usage
					`CREATE TABLE bandwidth_usage (
						satellite_id  BLOB    NOT NULL,
						action        INTEGER NOT NULL,
						amount        BIGINT  NOT NULL,
						created_at    TIMESTAMP NOT NULL
					)`,
					`CREATE INDEX idx_bandwidth_usage_satellite ON bandwidth_usage(satellite_id)`,
					`CREATE INDEX idx_bandwidth_usage_created   ON bandwidth_usage(created_at)`,

					// table for storing all unsent orders
					`CREATE TABLE unsent_order (
						satellite_id  BLOB NOT NULL,
						serial_number BLOB NOT NULL,

						order_limit_serialized BLOB      NOT NULL, -- serialized pb.OrderLimit
						order_serialized       BLOB      NOT NULL, -- serialized pb.Order
						order_limit_expiration TIMESTAMP NOT NULL, -- when is the deadline for sending it

						uplink_cert_id INTEGER NOT NULL,

						FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
					)`,
					`CREATE UNIQUE INDEX idx_orders ON unsent_order(satellite_id, serial_number)`,

					// table for storing all sent orders
					`CREATE TABLE order_archive (
						satellite_id  BLOB NOT NULL,
						serial_number BLOB NOT NULL,

						order_limit_serialized BLOB NOT NULL, -- serialized pb.OrderLimit
						order_serialized       BLOB NOT NULL, -- serialized pb.Order

						uplink_cert_id INTEGER NOT NULL,

						status      INTEGER   NOT NULL, -- accepted, rejected, confirmed
						archived_at TIMESTAMP NOT NULL, -- when was it rejected

						FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
					)`,
					`CREATE INDEX idx_order_archive_satellite ON order_archive(satellite_id)`,
					`CREATE INDEX idx_order_archive_status ON order_archive(status)`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Network Wipe #2",
				Version:     1,
				Action: migrate.SQL{
					`UPDATE pieceinfo SET piece_expiration = '2019-05-09 00:00:00.000000+00:00'`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Add tracking of deletion failures.",
				Version:     2,
				Action: migrate.SQL{
					`ALTER TABLE pieceinfo ADD COLUMN deletion_failed_at TIMESTAMP`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Add vouchersDB for storing and retrieving vouchers.",
				Version:     3,
				Action: migrate.SQL{
					`CREATE TABLE vouchers (
						satellite_id BLOB PRIMARY KEY NOT NULL,
						voucher_serialized BLOB NOT NULL,
						expiration TIMESTAMP NOT NULL
					)`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Add index on pieceinfo expireation",
				Version:     4,
				Action: migrate.SQL{
					`CREATE INDEX idx_pieceinfo_expiration ON pieceinfo(piece_expiration)`,
					`CREATE INDEX idx_pieceinfo_deletion_failed ON pieceinfo(deletion_failed_at)`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Partial Network Wipe - Tardigrade Satellites",
				Version:     5,
				Action: migrate.SQL{
					`UPDATE pieceinfo SET piece_expiration = '2019-06-25 00:00:00.000000+00:00' WHERE satellite_id
						IN (x'84A74C2CD43C5BA76535E1F42F5DF7C287ED68D33522782F4AFABFDB40000000',
							x'A28B4F04E10BAE85D67F4C6CB82BF8D4C0F0F47A8EA72627524DEB6EC0000000',
							x'AF2C42003EFC826AB4361F73F9D890942146FE0EBE806786F8E7190800000000'
					)`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Add creation date.",
				Version:     6,
				Action: migrate.SQL{
					`ALTER TABLE pieceinfo ADD COLUMN piece_creation TIMESTAMP NOT NULL DEFAULT 'epoch'`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Drop certificate table.",
				Version:     7,
				Action: migrate.SQL{
					`DROP TABLE certificate`,
					`CREATE TABLE certificate (cert_id INTEGER)`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Drop old used serials and remove pieceinfo_deletion_failed index.",
				Version:     8,
				Action: migrate.SQL{
					`DELETE FROM used_serial`,
					`DROP INDEX idx_pieceinfo_deletion_failed`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Add order limit table.",
				Version:     9,
				Action: migrate.SQL{
					`ALTER TABLE pieceinfo ADD COLUMN order_limit BLOB NOT NULL DEFAULT X''`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Optimize index usage.",
				Version:     10,
				Action: migrate.SQL{
					`DROP INDEX idx_pieceinfo_expiration`,
					`DROP INDEX idx_order_archive_satellite`,
					`DROP INDEX idx_order_archive_status`,
					`CREATE INDEX idx_pieceinfo_expiration ON pieceinfo(piece_expiration) WHERE piece_expiration IS NOT NULL`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Create bandwidth_usage_rollup table.",
				Version:     11,
				Action: migrate.SQL{
					`CREATE TABLE bandwidth_usage_rollups (
										interval_start	TIMESTAMP NOT NULL,
										satellite_id  	BLOB    NOT NULL,
										action        	INTEGER NOT NULL,
										amount        	BIGINT  NOT NULL,
										PRIMARY KEY ( interval_start, satellite_id, action )
									)`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Clear Tables from Alpha data",
				Version:     12,
				Action: migrate.SQL{
					`DROP TABLE pieceinfo`,
					`DROP TABLE used_serial`,
					`DROP TABLE order_archive`,
					`CREATE TABLE pieceinfo_ (
						satellite_id     BLOB      NOT NULL,
						piece_id         BLOB      NOT NULL,
						piece_size       BIGINT    NOT NULL,
						piece_expiration TIMESTAMP,

						order_limit       BLOB    NOT NULL,
						uplink_piece_hash BLOB    NOT NULL,
						uplink_cert_id    INTEGER NOT NULL,

						deletion_failed_at TIMESTAMP,
						piece_creation TIMESTAMP NOT NULL,

						FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
					)`,
					`CREATE UNIQUE INDEX pk_pieceinfo_ ON pieceinfo_(satellite_id, piece_id)`,
					`CREATE INDEX idx_pieceinfo__expiration ON pieceinfo_(piece_expiration) WHERE piece_expiration IS NOT NULL`,
					`CREATE TABLE used_serial_ (
						satellite_id  BLOB NOT NULL,
						serial_number BLOB NOT NULL,
						expiration    TIMESTAMP NOT NULL
					)`,
					`CREATE UNIQUE INDEX pk_used_serial_ ON used_serial_(satellite_id, serial_number)`,
					`CREATE INDEX idx_used_serial_ ON used_serial_(expiration)`,
					`CREATE TABLE order_archive_ (
						satellite_id  BLOB NOT NULL,
						serial_number BLOB NOT NULL,

						order_limit_serialized BLOB NOT NULL,
						order_serialized       BLOB NOT NULL,

						uplink_cert_id INTEGER NOT NULL,

						status      INTEGER   NOT NULL,
						archived_at TIMESTAMP NOT NULL,

						FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
					)`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Free Storagenodes from trash data",
				Version:     13,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, mgdb tagsql.DB, tx tagsql.Tx) error {
					err := os.RemoveAll(filepath.Join(db.dbDirectory, "blob/ukfu6bhbboxilvt7jrwlqk7y2tapb5d2r2tsmj2sjxvw5qaaaaaa")) // us-central1
					if err != nil {
						log.Debug("Error removing trash from us-central-1.", zap.Error(err))
					}
					err = os.RemoveAll(filepath.Join(db.dbDirectory, "blob/v4weeab67sbgvnbwd5z7tweqsqqun7qox2agpbxy44mqqaaaaaaa")) // europe-west1
					if err != nil {
						log.Debug("Error removing trash from europe-west-1.", zap.Error(err))
					}
					err = os.RemoveAll(filepath.Join(db.dbDirectory, "blob/qstuylguhrn2ozjv4h2c6xpxykd622gtgurhql2k7k75wqaaaaaa")) // asia-east1
					if err != nil {
						log.Debug("Error removing trash from asia-east-1.", zap.Error(err))
					}
					err = os.RemoveAll(filepath.Join(db.dbDirectory, "blob/abforhuxbzyd35blusvrifvdwmfx4hmocsva4vmpp3rgqaaaaaaa")) // "tothemoon (stefan)"
					if err != nil {
						log.Debug("Error removing trash from tothemoon.", zap.Error(err))
					}
					// To prevent the node from starting up, we just log errors and return nil
					return nil
				}),
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Free Storagenodes from orphaned tmp data",
				Version:     14,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, mgdb tagsql.DB, tx tagsql.Tx) error {
					err := os.RemoveAll(filepath.Join(db.dbDirectory, "tmp"))
					if err != nil {
						log.Debug("Error removing orphaned tmp data.", zap.Error(err))
					}
					// To prevent the node from starting up, we just log errors and return nil
					return nil
				}),
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Start piece_expirations table, deprecate pieceinfo table",
				Version:     15,
				Action: migrate.SQL{
					// new table to hold expiration data (and only expirations. no other pieceinfo)
					`CREATE TABLE piece_expirations (
						satellite_id       BLOB      NOT NULL,
						piece_id           BLOB      NOT NULL,
						piece_expiration   TIMESTAMP NOT NULL, -- date when it can be deleted
						deletion_failed_at TIMESTAMP,
						PRIMARY KEY (satellite_id, piece_id)
					)`,
					`CREATE INDEX idx_piece_expirations_piece_expiration ON piece_expirations(piece_expiration)`,
					`CREATE INDEX idx_piece_expirations_deletion_failed_at ON piece_expirations(deletion_failed_at)`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Add reputation and storage usage cache tables",
				Version:     16,
				Action: migrate.SQL{
					`CREATE TABLE reputation (
						satellite_id BLOB NOT NULL,
						uptime_success_count INTEGER NOT NULL,
						uptime_total_count INTEGER NOT NULL,
						uptime_reputation_alpha REAL NOT NULL,
						uptime_reputation_beta REAL NOT NULL,
						uptime_reputation_score REAL NOT NULL,
						audit_success_count INTEGER NOT NULL,
						audit_total_count INTEGER NOT NULL,
						audit_reputation_alpha REAL NOT NULL,
						audit_reputation_beta REAL NOT NULL,
						audit_reputation_score REAL NOT NULL,
						updated_at TIMESTAMP NOT NULL,
						PRIMARY KEY (satellite_id)
					)`,
					`CREATE TABLE storage_usage (
						satellite_id BLOB NOT NULL,
						at_rest_total REAL NOT NULL,
						timestamp TIMESTAMP NOT NULL,
						PRIMARY KEY (satellite_id, timestamp)
					)`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Create piece_space_used table",
				Version:     17,
				Action: migrate.SQL{
					// new table to hold the most recent totals from the piece space used cache
					`CREATE TABLE piece_space_used (
						total INTEGER NOT NULL,
						satellite_id BLOB
					)`,
					`CREATE UNIQUE INDEX idx_piece_space_used_satellite_id ON piece_space_used(satellite_id)`,
					`INSERT INTO piece_space_used (total) select ifnull(sum(piece_size), 0) from pieceinfo_`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Drop vouchers table",
				Version:     18,
				Action: migrate.SQL{
					`DROP TABLE vouchers`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Add disqualified field to reputation",
				Version:     19,
				Action: migrate.SQL{
					`DROP TABLE reputation;`,
					`CREATE TABLE reputation (
						satellite_id BLOB NOT NULL,
						uptime_success_count INTEGER NOT NULL,
						uptime_total_count INTEGER NOT NULL,
						uptime_reputation_alpha REAL NOT NULL,
						uptime_reputation_beta REAL NOT NULL,
						uptime_reputation_score REAL NOT NULL,
						audit_success_count INTEGER NOT NULL,
						audit_total_count INTEGER NOT NULL,
						audit_reputation_alpha REAL NOT NULL,
						audit_reputation_beta REAL NOT NULL,
						audit_reputation_score REAL NOT NULL,
						disqualified TIMESTAMP,
						updated_at TIMESTAMP NOT NULL,
						PRIMARY KEY (satellite_id)
					);`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Empty storage_usage table, rename storage_usage.timestamp to interval_start",
				Version:     20,
				Action: migrate.SQL{
					`DROP TABLE storage_usage`,
					`CREATE TABLE storage_usage (
						satellite_id BLOB NOT NULL,
						at_rest_total REAL NOT NULL,
						interval_start TIMESTAMP NOT NULL,
						PRIMARY KEY (satellite_id, interval_start)
					)`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Create satellites table and satellites_exit_progress table",
				Version:     21,
				Action: migrate.SQL{
					`CREATE TABLE satellites (
						node_id BLOB NOT NULL,
						address TEXT NOT NULL,
						added_at TIMESTAMP NOT NULL,
						status INTEGER NOT NULL,
						PRIMARY KEY (node_id)
					)`,
					`CREATE TABLE satellite_exit_progress (
						satellite_id BLOB NOT NULL,
						initiated_at TIMESTAMP,
						finished_at TIMESTAMP,
						starting_disk_usage INTEGER NOT NULL,
						bytes_deleted INTEGER NOT NULL,
						completion_receipt BLOB,
						PRIMARY KEY (satellite_id)
					)`,
				},
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Vacuum info db",
				Version:     22,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					_, err := db.deprecatedInfoDB.GetDB().ExecContext(ctx, "VACUUM;")
					return err
				}),
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Split into multiple sqlite databases",
				Version:     23,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					// Migrate all the tables to new database files.
					if err := db.migrateToDB(ctx, BandwidthDBName, "bandwidth_usage", "bandwidth_usage_rollups"); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.migrateToDB(ctx, OrdersDBName, "unsent_order", "order_archive_"); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.migrateToDB(ctx, PieceExpirationDBName, "piece_expirations"); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.migrateToDB(ctx, PieceInfoDBName, "pieceinfo_"); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.migrateToDB(ctx, PieceSpaceUsedDBName, "piece_space_used"); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.migrateToDB(ctx, ReputationDBName, "reputation"); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.migrateToDB(ctx, StorageUsageDBName, "storage_usage"); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.migrateToDB(ctx, UsedSerialsDBName, "used_serial_"); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.migrateToDB(ctx, SatellitesDBName, "satellites", "satellite_exit_progress"); err != nil {
						return ErrDatabase.Wrap(err)
					}

					return nil
				}),
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Drop unneeded tables in deprecatedInfoDB",
				Version:     24,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					// We drop the migrated tables from the deprecated database and VACUUM SQLite3
					// in migration step 23 because if we were to keep that as part of step 22
					// and an error occurred it would replay the entire migration but some tables
					// may have successfully dropped and we would experience unrecoverable data loss.
					// This way if step 22 completes it never gets replayed even if a drop table or
					// VACUUM call fails.
					if err := sqliteutil.KeepTables(ctx, db.rawDatabaseFromName(DeprecatedInfoDBName), VersionTable); err != nil {
						return ErrDatabase.Wrap(err)
					}

					return nil
				}),
			},
			{
				DB:          db.satellitesDB,
				Description: "Remove address from satellites table",
				Version:     25,
				Action: migrate.SQL{
					`CREATE TABLE satellites_new (
						node_id BLOB NOT NULL,
						added_at TIMESTAMP NOT NULL,
						status INTEGER NOT NULL,
						PRIMARY KEY (node_id)
					);
					INSERT INTO satellites_new (node_id, added_at, status)
						SELECT node_id, added_at, status
						FROM satellites;
					DROP TABLE satellites;
					ALTER TABLE satellites_new RENAME TO satellites;
					`,
				},
			},
			{
				DB:          db.pieceExpirationDB,
				Description: "Add Trash column to pieceExpirationDB",
				Version:     26,
				Action: migrate.SQL{
					`ALTER TABLE piece_expirations ADD COLUMN trash INTEGER NOT NULL DEFAULT 0`,
					`CREATE INDEX idx_piece_expirations_trashed
						ON piece_expirations(satellite_id, trash)
						WHERE trash = 1`,
				},
			},
			{
				DB:          db.ordersDB,
				Description: "Add index archived_at to ordersDB",
				Version:     27,
				Action: migrate.SQL{
					`CREATE INDEX idx_order_archived_at ON order_archive_(archived_at)`,
				},
			},
			{
				DB:          db.notificationsDB,
				Description: "Create notifications table",
				Version:     28,
				Action: migrate.SQL{
					`CREATE TABLE notifications (
						id         BLOB NOT NULL,
						sender_id  BLOB NOT NULL,
						type       INTEGER NOT NULL,
						title      TEXT NOT NULL,
						message    TEXT NOT NULL,
						read_at    TIMESTAMP,
						created_at TIMESTAMP NOT NULL,
						PRIMARY KEY (id)
					);`,
				},
			},
			{
				DB:          db.pieceSpaceUsedDB,
				Description: "Migrate piece_space_used to add total column",
				Version:     29,
				Action: migrate.SQL{
					`
					CREATE TABLE piece_space_used_new (
						total INTEGER NOT NULL DEFAULT 0,
						content_size INTEGER NOT NULL,
						satellite_id BLOB
					);
					INSERT INTO piece_space_used_new (content_size, satellite_id)
						SELECT total, satellite_id
						FROM piece_space_used;
					DROP TABLE piece_space_used;
					ALTER TABLE piece_space_used_new RENAME TO piece_space_used;
					`,
					`CREATE UNIQUE INDEX idx_piece_space_used_satellite_id ON piece_space_used(satellite_id)`,
				},
			},
			{
				DB:          db.pieceSpaceUsedDB,
				Description: "Initialize piece_space_used total column to content_size",
				Version:     30,
				Action: migrate.SQL{
					`UPDATE piece_space_used SET total = content_size`,
				},
			},
			{
				DB:          db.pieceSpaceUsedDB,
				Description: "Remove all 0 values from piece_space_used",
				Version:     31,
				Action: migrate.SQL{
					`UPDATE piece_space_used SET total = 0 WHERE total < 0`,
					`UPDATE piece_space_used SET content_size = 0 WHERE content_size < 0`,
				},
			},
			{
				DB:          db.heldamountDB,
				Description: "Create paystubs table and payments table",
				Version:     32,
				Action: migrate.SQL{
					`CREATE TABLE paystubs (
						period text NOT NULL,
						satellite_id bytea NOT NULL,
						created_at timestamp with time zone NOT NULL,
						codes text NOT NULL,
						usage_at_rest double precision NOT NULL,
						usage_get bigint NOT NULL,
						usage_put bigint NOT NULL,
						usage_get_repair bigint NOT NULL,
						usage_put_repair bigint NOT NULL,
						usage_get_audit bigint NOT NULL,
						comp_at_rest bigint NOT NULL,
						comp_get bigint NOT NULL,
						comp_put bigint NOT NULL,
						comp_get_repair bigint NOT NULL,
						comp_put_repair bigint NOT NULL,
						comp_get_audit bigint NOT NULL,
						surge_percent bigint NOT NULL,
						held bigint NOT NULL,
						owed bigint NOT NULL,
						disposed bigint NOT NULL,
						paid bigint NOT NULL,
						PRIMARY KEY ( period, satellite_id )
					);`,
					`CREATE TABLE payments (
						id bigserial NOT NULL,
						created_at timestamp with time zone NOT NULL,
						satellite_id bytea NOT NULL,
						period text,
						amount bigint NOT NULL,
						receipt text,
						notes text,
						PRIMARY KEY ( id )
					);`,
				},
			},
			{
				DB:          db.heldamountDB,
				Description: "Remove time zone from created_at in paystubs and payments",
				Version:     33,
				Action: migrate.SQL{
					`DROP TABLE paystubs;`,
					`DROP TABLE payments;`,
					`CREATE TABLE paystubs (
						period text NOT NULL,
						satellite_id bytea NOT NULL,
						created_at timestamp NOT NULL,
						codes text NOT NULL,
						usage_at_rest double precision NOT NULL,
						usage_get bigint NOT NULL,
						usage_put bigint NOT NULL,
						usage_get_repair bigint NOT NULL,
						usage_put_repair bigint NOT NULL,
						usage_get_audit bigint NOT NULL,
						comp_at_rest bigint NOT NULL,
						comp_get bigint NOT NULL,
						comp_put bigint NOT NULL,
						comp_get_repair bigint NOT NULL,
						comp_put_repair bigint NOT NULL,
						comp_get_audit bigint NOT NULL,
						surge_percent bigint NOT NULL,
						held bigint NOT NULL,
						owed bigint NOT NULL,
						disposed bigint NOT NULL,
						paid bigint NOT NULL,
						PRIMARY KEY ( period, satellite_id )
					);`,
					`CREATE TABLE payments (
						id bigserial NOT NULL,
						created_at timestamp NOT NULL,
						satellite_id bytea NOT NULL,
						period text,
						amount bigint NOT NULL,
						receipt text,
						notes text,
						PRIMARY KEY ( id )
					);`,
				},
			},
			{
				DB:          db.reputationDB,
				Description: "Add suspended field to satellites db",
				Version:     34,
				Action: migrate.SQL{
					`ALTER TABLE reputation ADD COLUMN suspended TIMESTAMP`,
				},
			},
			{
				DB:          db.pricingDB,
				Description: "Create pricing table",
				Version:     35,
				Action: migrate.SQL{
					`CREATE TABLE pricing (
						satellite_id BLOB NOT NULL,
						egress_bandwidth_price bigint NOT NULL,
						repair_bandwidth_price bigint NOT NULL,
						audit_bandwidth_price bigint NOT NULL,
						disk_space_price bigint NOT NULL,
						PRIMARY KEY ( satellite_id )
					);`,
				},
			},
			{
				DB:          db.reputationDB,
				Description: "Add joined_at field to satellites db",
				Version:     36,
				Action: migrate.SQL{
					`ALTER TABLE reputation ADD COLUMN joined_at TIMESTAMP`,
				},
			},
			{
				DB:          db.heldamountDB,
				Description: "Drop payments table as unused",
				Version:     37,
				Action: migrate.SQL{
					`DROP TABLE payments;`,
				},
			},
			{
				DB:          db.reputationDB,
				Description: "Backfill joined_at column",
				Version:     38,
				Action: migrate.Func(func(ctx context.Context, _ *zap.Logger, rdb tagsql.DB, rtx tagsql.Tx) (err error) {
					// We just need a value for joined_at until the node checks in with the
					// satellites and gets the real value.
					_, err = rtx.Exec(ctx, `UPDATE reputation SET joined_at = ? WHERE joined_at ISNULL`, time.Unix(0, 0).UTC())
					if err != nil {
						return errs.Wrap(err)
					}

					// in order to add the not null constraint, we have to do a
					// generalized ALTER TABLE procedure.
					// see https://www.sqlite.org/lang_altertable.html
					_, err = rtx.Exec(ctx, `
						CREATE TABLE reputation_new (
							satellite_id BLOB NOT NULL,
							uptime_success_count INTEGER NOT NULL,
							uptime_total_count INTEGER NOT NULL,
							uptime_reputation_alpha REAL NOT NULL,
							uptime_reputation_beta REAL NOT NULL,
							uptime_reputation_score REAL NOT NULL,
							audit_success_count INTEGER NOT NULL,
							audit_total_count INTEGER NOT NULL,
							audit_reputation_alpha REAL NOT NULL,
							audit_reputation_beta REAL NOT NULL,
							audit_reputation_score REAL NOT NULL,
							disqualified TIMESTAMP,
							updated_at TIMESTAMP NOT NULL,
							suspended TIMESTAMP,
							joined_at TIMESTAMP NOT NULL,
							PRIMARY KEY (satellite_id)
						);
						INSERT INTO reputation_new SELECT
							satellite_id,
							uptime_success_count,
							uptime_total_count,
							uptime_reputation_alpha,
							uptime_reputation_beta,
							uptime_reputation_score,
							audit_success_count,
							audit_total_count,
							audit_reputation_alpha,
							audit_reputation_beta,
							audit_reputation_score,
							disqualified,
							updated_at,
							suspended,
							joined_at
							FROM reputation;
						DROP TABLE reputation;
						ALTER TABLE reputation_new RENAME TO reputation;
					`)
					if err != nil {
						return errs.Wrap(err)
					}

					return nil
				}),
			},
			{
				DB:          db.reputationDB,
				Description: "Add unknown_audit_reputation_alpha and unknown_audit_reputation_beta fields to satellites db and remove uptime_reputation_alpha, uptime_reputation_beta, uptime_reputation_score",
				Version:     39,
				Action: migrate.Func(func(ctx context.Context, _ *zap.Logger, rdb tagsql.DB, rtx tagsql.Tx) (err error) {
					_, err = rtx.Exec(ctx, `ALTER TABLE reputation ADD COLUMN audit_unknown_reputation_alpha REAL`)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.Exec(ctx, `ALTER TABLE reputation ADD COLUMN audit_unknown_reputation_beta REAL`)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.Exec(ctx, `UPDATE reputation SET audit_unknown_reputation_alpha = ?, audit_unknown_reputation_beta = ?`,
						1.0, 1.0)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.Exec(ctx, `
						CREATE TABLE reputation_new (
							satellite_id BLOB NOT NULL,
							uptime_success_count INTEGER NOT NULL,
							uptime_total_count INTEGER NOT NULL,
							uptime_reputation_alpha REAL NOT NULL,
							uptime_reputation_beta REAL NOT NULL,
							uptime_reputation_score REAL NOT NULL,
							audit_success_count INTEGER NOT NULL,
							audit_total_count INTEGER NOT NULL,
							audit_reputation_alpha REAL NOT NULL,
							audit_reputation_beta REAL NOT NULL,
							audit_reputation_score REAL NOT NULL,
							audit_unknown_reputation_alpha REAL NOT NULL,
							audit_unknown_reputation_beta REAL NOT NULL,
							disqualified TIMESTAMP,
							updated_at TIMESTAMP NOT NULL,
							suspended TIMESTAMP,
							joined_at TIMESTAMP NOT NULL,
							PRIMARY KEY (satellite_id)
						);
						INSERT INTO reputation_new SELECT
							satellite_id,
							uptime_success_count,
							uptime_total_count,
							uptime_reputation_alpha,
							uptime_reputation_beta,
							uptime_reputation_score,
							audit_success_count,
							audit_total_count,
							audit_reputation_alpha,
							audit_reputation_beta,
							audit_reputation_score,
							audit_unknown_reputation_alpha,
							audit_unknown_reputation_beta,
							disqualified,
							updated_at,
							suspended,
							joined_at
							FROM reputation;
						DROP TABLE reputation;
						ALTER TABLE reputation_new RENAME TO reputation;
					`)
					if err != nil {
						return errs.Wrap(err)
					}

					return nil
				}),
			},
		},
	}
}
