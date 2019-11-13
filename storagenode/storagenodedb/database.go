// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3" // used indirectly.
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/dbutil"
	"storj.io/storj/internal/dbutil/sqliteutil"
	"storj.io/storj/internal/migrate"
	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
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
)

var _ storagenode.DB = (*DB)(nil)

// SQLDB defines an interface to allow accessing and setting an sql.DB
type SQLDB interface {
	Configure(sqlDB *sql.DB)
	GetDB() *sql.DB
}

// withTx is a helper method which executes callback in transaction scope
func withTx(ctx context.Context, db *sql.DB, cb func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
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
	Storage string
	Info    string
	Info2   string

	Pieces string
}

// DB contains access to different database tables
type DB struct {
	log *zap.Logger

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

	sqlDatabases map[string]SQLDB
}

// New creates a new master database for storage node
func New(log *zap.Logger, config Config) (*DB, error) {
	piecesDir, err := filestore.NewDir(config.Pieces)
	if err != nil {
		return nil, err
	}
	pieces := filestore.New(log, piecesDir)

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

	db := &DB{
		log:    log,
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

		sqlDatabases: map[string]SQLDB{
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
	return nil
}

func (db *DB) rawDatabaseFromName(dbName string) *sql.DB {
	return db.sqlDatabases[dbName].GetDB()
}

// openDatabase opens or creates a database at the specified path.
func (db *DB) openDatabase(dbName string) error {
	path := db.filepathFromDBName(dbName)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return ErrDatabase.Wrap(err)
	}

	sqlDB, err := sql.Open("sqlite3", "file:"+path+"?_journal=WAL&_busy_timeout=10000")
	if err != nil {
		return ErrDatabase.Wrap(err)
	}

	mDB := db.sqlDatabases[dbName]
	mDB.Configure(sqlDB)

	dbutil.Configure(sqlDB, mon)

	db.log.Debug(fmt.Sprintf("opened database %s", dbName))
	return nil
}

// filenameFromDBName returns a constructed filename for the specified database name.
func (db *DB) filenameFromDBName(dbName string) string {
	return dbName + ".db"
}

func (db *DB) filepathFromDBName(dbName string) string {
	return filepath.Join(db.dbDirectory, db.filenameFromDBName(dbName))
}

// CreateTables creates any necessary tables.
func (db *DB) CreateTables(ctx context.Context) error {
	migration := db.Migration(ctx)
	return migration.Run(db.log.Named("migration"))
}

// Close closes any resources.
func (db *DB) Close() error {
	return db.closeDatabases()
}

// closeDatabases closes all the SQLite database connections and removes them from the associated maps.
func (db *DB) closeDatabases() error {
	var errlist errs.Group

	for k := range db.sqlDatabases {
		errlist.Add(db.closeDatabase(k))
	}
	return errlist.Err()
}

// closeDatabase closes the specified SQLite database connections and removes them from the associated maps.
func (db *DB) closeDatabase(dbName string) (err error) {
	mdb, ok := db.sqlDatabases[dbName]
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

// RawDatabases are required for testing purposes
func (db *DB) RawDatabases() map[string]SQLDB {
	return db.sqlDatabases
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

	err = sqliteutil.MigrateTablesToDatabase(ctx, db.rawDatabaseFromName(DeprecatedInfoDBName), db.rawDatabaseFromName(dbName), tablesToKeep...)
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
				Action: migrate.Func(func(log *zap.Logger, mgdb migrate.DB, tx *sql.Tx) error {
					err := os.RemoveAll(filepath.Join(db.dbDirectory, "blob/ukfu6bhbboxilvt7jrwlqk7y2tapb5d2r2tsmj2sjxvw5qaaaaaa")) // us-central1
					if err != nil {
						log.Sugar().Debug(err)
					}
					err = os.RemoveAll(filepath.Join(db.dbDirectory, "blob/v4weeab67sbgvnbwd5z7tweqsqqun7qox2agpbxy44mqqaaaaaaa")) // europe-west1
					if err != nil {
						log.Sugar().Debug(err)
					}
					err = os.RemoveAll(filepath.Join(db.dbDirectory, "blob/qstuylguhrn2ozjv4h2c6xpxykd622gtgurhql2k7k75wqaaaaaa")) // asia-east1
					if err != nil {
						log.Sugar().Debug(err)
					}
					err = os.RemoveAll(filepath.Join(db.dbDirectory, "blob/abforhuxbzyd35blusvrifvdwmfx4hmocsva4vmpp3rgqaaaaaaa")) // "tothemoon (stefan)"
					if err != nil {
						log.Sugar().Debug(err)
					}
					// To prevent the node from starting up, we just log errors and return nil
					return nil
				}),
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Free Storagenodes from orphaned tmp data",
				Version:     14,
				Action: migrate.Func(func(log *zap.Logger, mgdb migrate.DB, tx *sql.Tx) error {
					err := os.RemoveAll(filepath.Join(db.dbDirectory, "tmp"))
					if err != nil {
						log.Sugar().Debug(err)
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
				Action: migrate.Func(func(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
					_, err := db.deprecatedInfoDB.GetDB().Exec("VACUUM;")
					return err
				}),
			},
			{
				DB:          db.deprecatedInfoDB,
				Description: "Split into multiple sqlite databases",
				Version:     23,
				Action: migrate.Func(func(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
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
				Action: migrate.Func(func(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
					// We drop the migrated tables from the deprecated database and VACUUM SQLite3
					// in migration step 23 because if we were to keep that as part of step 22
					// and an error occurred it would replay the entire migration but some tables
					// may have successfully dropped and we would experience unrecoverable data loss.
					// This way if step 22 completes it never gets replayed even if a drop table or
					// VACUUM call fails.
					if err := sqliteutil.KeepTables(ctx, db.rawDatabaseFromName(DeprecatedInfoDBName), VersionTable); err != nil {
						return ErrDatabase.Wrap(err)
					}

					// Close the deprecated db in order to free up unused
					// disk space
					if err := db.closeDatabase(DeprecatedInfoDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}

					if err := db.openDatabase(DeprecatedInfoDBName); err != nil {
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
					`ALTER TABLE satellites RENAME TO _satellites_old`,
					`CREATE TABLE satellites (
						node_id BLOB NOT NULL,
						added_at TIMESTAMP NOT NULL,
						status INTEGER NOT NULL,
						PRIMARY KEY (node_id)
					)`,
					`INSERT INTO satellites (node_id, added_at, status)
						SELECT node_id, added_at, status
						FROM _satellites_old`,
					`DROP TABLE _satellites_old`,
				},
			},
		},
	}
}
