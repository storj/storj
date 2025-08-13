// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:generate go run ./schemagen -o schema.go

package storagenodedb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-cmp/cmp"
	_ "github.com/mattn/go-sqlite3" // used indirectly.
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/storj/private/migrate"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/dbschema"
	"storj.io/storj/shared/dbutil/sqliteutil"
	"storj.io/storj/shared/tagsql"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/blobstore/statcache"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/payouts"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/storageusage"
)

// VersionTable is the table that stores the version info in each db.
const VersionTable = "versions"

var (
	mon = monkit.Package()

	// ErrDatabase represents errors from the databases.
	ErrDatabase = errs.Class("database")
	// ErrNoRows represents database error if rows weren't affected.
	ErrNoRows = errs.New("no rows affected")
	// ErrPreflight represents an error during the preflight check.
	ErrPreflight = errs.Class("preflight")
)

// DBContainer defines an interface to allow accessing and setting a SQLDB.
type DBContainer interface {
	Configure(sqlDB tagsql.DB)
	GetDB() tagsql.DB
}

// Config configures storage node database.
type Config struct {
	// TODO: figure out better names
	Storage   string
	Cache     string
	Info      string
	Info2     string
	Driver    string // if unset, uses sqlite3
	Pieces    string
	Filestore filestore.Config

	TestingDisableWAL bool
}

// LazyFilewalkerConfig creates a lazyfilewalker.Config from storagenodedb.Config.
//
// TODO: this is a temporary solution to avoid circular dependencies.
func (config Config) LazyFilewalkerConfig() lazyfilewalker.Config {
	return lazyfilewalker.Config{
		Storage:         config.Storage,
		Info:            config.Info,
		Info2:           config.Info2,
		Driver:          config.Driver,
		Pieces:          config.Pieces,
		Filestore:       config.Filestore,
		Cache:           config.Cache,
		LowerIOPriority: true,
	}
}

// DB contains access to different database tables.
type DB struct {
	log    *zap.Logger
	config Config

	pieces blobstore.Blobs

	dbDirectory string

	deprecatedInfoDB       *deprecatedInfoDB
	v0PieceInfoDB          *v0PieceInfoDB
	bandwidthDB            *BandwidthDB
	ordersDB               *ordersDB
	pieceExpirationDB      *pieceExpirationDB
	pieceSpaceUsedDB       *pieceSpaceUsedDB
	reputationDB           *reputationDB
	storageUsageDB         *storageUsageDB
	usedSerialsDB          *usedSerialsDB
	satellitesDB           *satellitesDB
	notificationsDB        *notificationDB
	payoutDB               *payoutDB
	pricingDB              *pricingDB
	apiKeysDB              *apiKeysDB
	gcFilewalkerProgressDB *gcFilewalkerProgressDB
	usedSpacePerPrefixDB   *usedSpacePerPrefixDB

	SQLDBs map[string]DBContainer

	cache statcache.Cache
}

// OpenNew creates a new master database for storage node.
func OpenNew(ctx context.Context, log *zap.Logger, config Config) (*DB, error) {
	piecesDir, err := filestore.NewDir(log, config.Pieces)
	if err != nil {
		return nil, err
	}

	pieces := filestore.New(log, piecesDir, config.Filestore)

	var cache statcache.Cache
	pieces, cache, err = cachedBlobstore(log, pieces, config)
	if err != nil {
		return nil, err
	}

	deprecatedInfoDB := &deprecatedInfoDB{}
	v0PieceInfoDB := &v0PieceInfoDB{}
	bandwidthDB := &BandwidthDB{}
	ordersDB := &ordersDB{}
	pieceExpirationDB := &pieceExpirationDB{}
	pieceSpaceUsedDB := &pieceSpaceUsedDB{}
	reputationDB := &reputationDB{}
	storageUsageDB := &storageUsageDB{}
	usedSerialsDB := &usedSerialsDB{}
	satellitesDB := &satellitesDB{}
	notificationsDB := &notificationDB{}
	payoutDB := &payoutDB{}
	pricingDB := &pricingDB{}
	apiKeysDB := &apiKeysDB{}
	gcFilewalkerProgressDB := &gcFilewalkerProgressDB{}
	usedSpacePerPrefixDB := &usedSpacePerPrefixDB{}

	db := &DB{
		log:    log,
		config: config,

		pieces: pieces,

		cache: cache,

		dbDirectory: filepath.Dir(config.Info2),

		deprecatedInfoDB:       deprecatedInfoDB,
		v0PieceInfoDB:          v0PieceInfoDB,
		bandwidthDB:            bandwidthDB,
		ordersDB:               ordersDB,
		pieceExpirationDB:      pieceExpirationDB,
		pieceSpaceUsedDB:       pieceSpaceUsedDB,
		reputationDB:           reputationDB,
		storageUsageDB:         storageUsageDB,
		usedSerialsDB:          usedSerialsDB,
		satellitesDB:           satellitesDB,
		notificationsDB:        notificationsDB,
		payoutDB:               payoutDB,
		pricingDB:              pricingDB,
		apiKeysDB:              apiKeysDB,
		gcFilewalkerProgressDB: gcFilewalkerProgressDB,
		usedSpacePerPrefixDB:   usedSpacePerPrefixDB,

		SQLDBs: map[string]DBContainer{
			DeprecatedInfoDBName:       deprecatedInfoDB,
			PieceInfoDBName:            v0PieceInfoDB,
			BandwidthDBName:            bandwidthDB,
			OrdersDBName:               ordersDB,
			PieceExpirationDBName:      pieceExpirationDB,
			PieceSpaceUsedDBName:       pieceSpaceUsedDB,
			ReputationDBName:           reputationDB,
			StorageUsageDBName:         storageUsageDB,
			UsedSerialsDBName:          usedSerialsDB,
			SatellitesDBName:           satellitesDB,
			NotificationsDBName:        notificationsDB,
			HeldAmountDBName:           payoutDB,
			PricingDBName:              pricingDB,
			APIKeysDBName:              apiKeysDB,
			GCFilewalkerProgressDBName: gcFilewalkerProgressDB,
			UsedSpacePerPrefixDBName:   usedSpacePerPrefixDB,
		},
	}

	return db, nil
}

func cachedBlobstore(log *zap.Logger, blobs blobstore.Blobs, config Config) (blobstore.Blobs, statcache.Cache, error) {
	switch config.Cache {
	case "":
		return blobs, nil, nil
	case "badger":
		flog := process.NamedLog(log, "filestatcache")
		cache, err := statcache.NewBadgerCache(flog, filepath.Join(config.Storage, "filestatcache"))
		if err != nil {
			return nil, nil, errs.Wrap(err)
		}
		return statcache.NewCachedStatBlobStore(flog, cache, blobs), cache, nil

	default:
		return nil, nil, errs.New("Unknown file stat cache: %s", config.Cache)
	}
}

// OpenExisting opens an existing master database for storage node.
func OpenExisting(ctx context.Context, log *zap.Logger, config Config) (*DB, error) {
	piecesDir, err := filestore.OpenDir(log, config.Pieces, time.Now())
	if err != nil {
		return nil, err
	}

	pieces := filestore.New(log, piecesDir, config.Filestore)

	var cache statcache.Cache
	pieces, cache, err = cachedBlobstore(log, pieces, config)
	if err != nil {
		return nil, err
	}

	deprecatedInfoDB := &deprecatedInfoDB{}
	v0PieceInfoDB := &v0PieceInfoDB{}
	bandwidthDB := &BandwidthDB{}
	ordersDB := &ordersDB{}
	pieceExpirationDB := &pieceExpirationDB{}
	pieceSpaceUsedDB := &pieceSpaceUsedDB{}
	reputationDB := &reputationDB{}
	storageUsageDB := &storageUsageDB{}
	usedSerialsDB := &usedSerialsDB{}
	satellitesDB := &satellitesDB{}
	notificationsDB := &notificationDB{}
	payoutDB := &payoutDB{}
	pricingDB := &pricingDB{}
	apiKeysDB := &apiKeysDB{}
	gcFilewalkerProgressDB := &gcFilewalkerProgressDB{}
	usedSpacePerPrefixDB := &usedSpacePerPrefixDB{}

	db := &DB{
		log:    log,
		config: config,

		pieces: pieces,

		cache: cache,

		dbDirectory: filepath.Dir(config.Info2),

		deprecatedInfoDB:       deprecatedInfoDB,
		v0PieceInfoDB:          v0PieceInfoDB,
		bandwidthDB:            bandwidthDB,
		ordersDB:               ordersDB,
		pieceExpirationDB:      pieceExpirationDB,
		pieceSpaceUsedDB:       pieceSpaceUsedDB,
		reputationDB:           reputationDB,
		storageUsageDB:         storageUsageDB,
		usedSerialsDB:          usedSerialsDB,
		satellitesDB:           satellitesDB,
		notificationsDB:        notificationsDB,
		payoutDB:               payoutDB,
		pricingDB:              pricingDB,
		apiKeysDB:              apiKeysDB,
		gcFilewalkerProgressDB: gcFilewalkerProgressDB,
		usedSpacePerPrefixDB:   usedSpacePerPrefixDB,

		SQLDBs: map[string]DBContainer{
			DeprecatedInfoDBName:       deprecatedInfoDB,
			PieceInfoDBName:            v0PieceInfoDB,
			BandwidthDBName:            bandwidthDB,
			OrdersDBName:               ordersDB,
			PieceExpirationDBName:      pieceExpirationDB,
			PieceSpaceUsedDBName:       pieceSpaceUsedDB,
			ReputationDBName:           reputationDB,
			StorageUsageDBName:         storageUsageDB,
			UsedSerialsDBName:          usedSerialsDB,
			SatellitesDBName:           satellitesDB,
			NotificationsDBName:        notificationsDB,
			HeldAmountDBName:           payoutDB,
			PricingDBName:              pricingDB,
			APIKeysDBName:              apiKeysDB,
			GCFilewalkerProgressDBName: gcFilewalkerProgressDB,
			UsedSpacePerPrefixDBName:   usedSpacePerPrefixDB,
		},
	}

	err = db.openDatabases(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// openDatabases opens all the SQLite3 storage node databases and returns if any fails to open successfully.
func (db *DB) openDatabases(ctx context.Context) error {
	// These objects have a Configure method to allow setting the underlining SQLDB connection
	// that each uses internally to do data access to the SQLite3 databases.
	// The reason it was done this way was because there's some outside consumers that are
	// taking a reference to the business object.

	dbs := []string{
		DeprecatedInfoDBName,
		BandwidthDBName,
		OrdersDBName,
		PieceExpirationDBName,
		PieceInfoDBName,
		PieceSpaceUsedDBName,
		ReputationDBName,
		StorageUsageDBName,
		UsedSerialsDBName,
		SatellitesDBName,
		NotificationsDBName,
		HeldAmountDBName,
		PricingDBName,
		APIKeysDBName,
		GCFilewalkerProgressDBName,
		UsedSpacePerPrefixDBName,
	}

	for _, dbName := range dbs {
		err := db.openExistingDatabase(ctx, dbName, true)
		if err != nil {
			return errs.Combine(err, db.closeDatabases())
		}
	}

	return nil
}

func (db *DB) rawDatabaseFromName(dbName string) tagsql.DB {
	return db.SQLDBs[dbName].GetDB()
}

// openExistingDatabase opens existing database at the specified path.
func (db *DB) openExistingDatabase(ctx context.Context, dbName string, withStat bool) error {
	path := db.filepathFromDBName(dbName)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			// When we haven't created the database, yet, then stat fails.
			db.log.Info("database does not exist", zap.String("database", dbName))
			return nil
		}
		return ErrDatabase.New("%s couldn't be read (%q): %w", dbName, path, err)
	}

	return db.openDatabaseWithStat(ctx, dbName, withStat)
}

// openDatabase opens or creates a database at the specified path.
func (db *DB) openDatabase(ctx context.Context, dbName string) error {
	return db.openDatabaseWithStat(ctx, dbName, false)
}

// openDatabase opens or creates a database at the specified path.
func (db *DB) openDatabaseWithStat(ctx context.Context, dbName string, registerStat bool) error {
	path := db.filepathFromDBName(dbName)

	driver := db.config.Driver
	if driver == "" {
		driver = "sqlite3"
	}

	if err := db.closeDatabase(dbName); err != nil {
		return ErrDatabase.Wrap(err)
	}

	wal := "&_journal=WAL"
	if db.config.TestingDisableWAL {
		wal = "&_journal=MEMORY&_txlock=immediate"
	}

	sqlDB, err := tagsql.Open(ctx, driver, "file:"+path+"?_busy_timeout=10000"+wal, nil)
	if err != nil {
		return ErrDatabase.New("%s opening file %q failed: %w", dbName, path, err)
	}

	mDB := db.SQLDBs[dbName]
	mDB.Configure(sqlDB)

	if db.config.TestingDisableWAL {
		sqlDB.SetMaxOpenConns(1)
	}

	if registerStat {
		dbutil.Configure(ctx, sqlDB, dbName, mon)
	}

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
	return migration.Run(ctx, process.NamedLog(db.log, "migration"))
}

// Preflight conducts a pre-flight check to ensure correct schemas and minimal read+write functionality of the database tables.
func (db *DB) Preflight(ctx context.Context) (err error) {
	for dbName, dbContainer := range db.SQLDBs {
		if err := db.preflight(ctx, dbName, dbContainer); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) preflight(ctx context.Context, dbName string, dbContainer DBContainer) error {
	nextDB := dbContainer.GetDB()
	// Preflight stage 1: test schema correctness
	schema, err := sqliteutil.QuerySchema(ctx, nextDB)
	if err != nil {
		return ErrPreflight.New("database %q: schema check failed: %v", dbName, err)
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

	// get expected schema
	expectedSchema := Schema()[dbName]

	// find extra indexes
	var extraIdxs []*dbschema.Index
	for _, idx := range schema.Indexes {
		if _, exists := expectedSchema.FindIndex(idx.Name); exists {
			continue
		}

		extraIdxs = append(extraIdxs, idx)
	}
	// drop index from schema if it is not unique to not fail preflight
	for _, idx := range extraIdxs {
		if !idx.Unique {
			schema.DropIndex(idx.Name)
		}
	}
	// warn that schema contains unexpected indexes
	if len(extraIdxs) > 0 {
		db.log.Warn(fmt.Sprintf("database %q: schema contains unexpected indices %v", dbName, extraIdxs))
	}

	// expect expected schema to match actual schema
	if diff := cmp.Diff(expectedSchema, schema); diff != "" {
		return ErrPreflight.New("database %q: expected schema does not match actual: %s", dbName, diff)
	}

	// Preflight stage 2: test basic read/write access
	// for each database, create a new table, insert a row into that table, retrieve and validate that row, and drop the table.

	// drop test table in case the last preflight check failed before table could be dropped
	_, err = nextDB.ExecContext(ctx, "DROP TABLE IF EXISTS test_table")
	if err != nil {
		return ErrPreflight.New("database %q: failed drop if test_table: %w", dbName, err)
	}
	_, err = nextDB.ExecContext(ctx, "CREATE TABLE test_table(id int NOT NULL, name varchar(30), PRIMARY KEY (id))")
	if err != nil {
		return ErrPreflight.New("database %q: failed create test_table: %w", dbName, err)
	}

	var expectedID, actualID int
	var expectedName, actualName string
	expectedID = 1
	expectedName = "TEST"
	_, err = nextDB.ExecContext(ctx, "INSERT INTO test_table VALUES ( ?, ? )", expectedID, expectedName)
	if err != nil {
		return ErrPreflight.New("database: %q: failed inserting test value: %w", dbName, err)
	}

	rows, err := nextDB.QueryContext(ctx, "SELECT id, name FROM test_table")
	if err != nil {
		return ErrPreflight.New("database: %q: failed selecting test value: %w", dbName, err)
	}
	defer func() { err = errs.Combine(err, rows.Err(), rows.Close()) }()
	if !rows.Next() {
		return ErrPreflight.New("database %q: no rows in test_table", dbName)
	}
	err = rows.Scan(&actualID, &actualName)
	if err != nil {
		return ErrPreflight.New("database %q: failed scanning row: %w", dbName, err)
	}
	if expectedID != actualID || expectedName != actualName {
		return ErrPreflight.New("database %q: expected (%d, '%s'), actual (%d, '%s')", dbName, expectedID, expectedName, actualID, actualName)
	}
	if rows.Next() {
		return ErrPreflight.New("database %q: more than one row in test_table", dbName)
	}

	_, err = nextDB.ExecContext(ctx, "DROP TABLE test_table")
	if err != nil {
		return ErrPreflight.New("database %q: failed drop test_table %w", dbName, err)
	}

	return nil
}

// Close closes any resources.
func (db *DB) Close() error {
	if db.cache != nil {
		_ = db.cache.Close()
	}
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
	// if an error occurred during openDatabase, there will be no internal DB to close
	dbHandle := mdb.GetDB()
	if dbHandle == nil {
		return nil
	}

	err = dbHandle.Close()
	if err != nil {
		return ErrDatabase.New("%s close failed: %w", dbName, err)
	}
	return nil
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

// Pieces returns blob storage for pieces.
func (db *DB) Pieces() blobstore.Blobs {
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

// Satellites returns the instance of the Satellites database.
func (db *DB) Satellites() satellites.DB {
	return db.satellitesDB
}

// Notifications returns the instance of the Notifications database.
func (db *DB) Notifications() notifications.DB {
	return db.notificationsDB
}

// Payout returns instance of the SnoPayout database.
func (db *DB) Payout() payouts.DB {
	return db.payoutDB
}

// Pricing returns instance of the Pricing database.
func (db *DB) Pricing() pricing.DB {
	return db.pricingDB
}

// APIKeys returns instance of the APIKeys database.
func (db *DB) APIKeys() apikeys.DB {
	return db.apiKeysDB
}

// GCFilewalkerProgress returns the instance of the GCFilewalkerProgress database.
func (db *DB) GCFilewalkerProgress() pieces.GCFilewalkerProgressDB {
	return db.gcFilewalkerProgressDB
}

// UsedSpacePerPrefix returns the instance of the UsedSpacePerPrefix database.
func (db *DB) UsedSpacePerPrefix() pieces.UsedSpacePerPrefixDB {
	return db.usedSpacePerPrefixDB
}

// RawDatabases are required for testing purposes.
func (db *DB) RawDatabases() map[string]DBContainer {
	return db.SQLDBs
}

// DBDirectory returns the database directory for testing purposes.
func (db *DB) DBDirectory() string {
	return db.dbDirectory
}

// Config returns the database configuration used to initialize the database.
func (db *DB) Config() Config {
	return db.config
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
			return ErrDatabase.New("%s failed to remove %q: %w", dbName, path, err)
		}
	}

	err = db.openDatabaseWithStat(ctx, dbName, false)
	if err != nil {
		return ErrDatabase.Wrap(err)
	}

	err = sqliteutil.MigrateTablesToDatabase(ctx,
		db.rawDatabaseFromName(DeprecatedInfoDBName),
		db.rawDatabaseFromName(dbName),
		tablesToKeep...)
	if err != nil {
		return ErrDatabase.New("%s migrate tables to database: %w", dbName, err)
	}

	// We need to close and re-open the database we have just migrated *to* in
	// order to recover any excess disk usage that was freed in the VACUUM call
	err = db.closeDatabase(dbName)
	if err != nil {
		return ErrDatabase.Wrap(err)
	}

	err = db.openExistingDatabase(ctx, dbName, false)
	if err != nil {
		return ErrDatabase.Wrap(err)
	}

	return nil
}

// CheckVersion that the version of the migration matches the state of the database.
func (db *DB) CheckVersion(ctx context.Context) error {
	return db.Migration(ctx).ValidateVersions(ctx, db.log)
}

// Migration returns table migrations.
func (db *DB) Migration(ctx context.Context) *migrate.Migration {
	return &migrate.Migration{
		Table: VersionTable,
		Steps: []*migrate.Step{
			{
				DB:          &db.deprecatedInfoDB.DB,
				Description: "Initial setup",
				Version:     0,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, DeprecatedInfoDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}

					return nil
				},
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
				DB:          &db.deprecatedInfoDB.DB,
				Description: "Network Wipe #2",
				Version:     1,
				Action: migrate.SQL{
					`UPDATE pieceinfo SET piece_expiration = '2019-05-09 00:00:00.000000+00:00'`,
				},
			},
			{
				DB:          &db.deprecatedInfoDB.DB,
				Description: "Add tracking of deletion failures.",
				Version:     2,
				Action: migrate.SQL{
					`ALTER TABLE pieceinfo ADD COLUMN deletion_failed_at TIMESTAMP`,
				},
			},
			{
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.deprecatedInfoDB.DB,
				Description: "Add index on pieceinfo expireation",
				Version:     4,
				Action: migrate.SQL{
					`CREATE INDEX idx_pieceinfo_expiration ON pieceinfo(piece_expiration)`,
					`CREATE INDEX idx_pieceinfo_deletion_failed ON pieceinfo(deletion_failed_at)`,
				},
			},
			{
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.deprecatedInfoDB.DB,
				Description: "Add creation date.",
				Version:     6,
				Action: migrate.SQL{
					`ALTER TABLE pieceinfo ADD COLUMN piece_creation TIMESTAMP NOT NULL DEFAULT 'epoch'`,
				},
			},
			{
				DB:          &db.deprecatedInfoDB.DB,
				Description: "Drop certificate table.",
				Version:     7,
				Action: migrate.SQL{
					`DROP TABLE certificate`,
					`CREATE TABLE certificate (cert_id INTEGER)`,
				},
			},
			{
				DB:          &db.deprecatedInfoDB.DB,
				Description: "Drop old used serials and remove pieceinfo_deletion_failed index.",
				Version:     8,
				Action: migrate.SQL{
					`DELETE FROM used_serial`,
					`DROP INDEX idx_pieceinfo_deletion_failed`,
				},
			},
			{
				DB:          &db.deprecatedInfoDB.DB,
				Description: "Add order limit table.",
				Version:     9,
				Action: migrate.SQL{
					`ALTER TABLE pieceinfo ADD COLUMN order_limit BLOB NOT NULL DEFAULT X''`,
				},
			},
			{
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.deprecatedInfoDB.DB,
				Description: "Create piece_space_used table",
				Version:     17,
				Action: migrate.SQL{
					// new table to hold the most recent totals from the piece space used cache
					`CREATE TABLE piece_space_used (
						total INTEGER NOT NULL,
						satellite_id BLOB
					)`,
					`CREATE UNIQUE INDEX idx_piece_space_used_satellite_id ON piece_space_used(satellite_id)`,
					`INSERT INTO piece_space_used (total) SELECT IFNULL(SUM(piece_size), 0) FROM pieceinfo_`,
				},
			},
			{
				DB:          &db.deprecatedInfoDB.DB,
				Description: "Drop vouchers table",
				Version:     18,
				Action: migrate.SQL{
					`DROP TABLE vouchers`,
				},
			},
			{
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.deprecatedInfoDB.DB,
				Description: "Vacuum info db",
				Version:     22,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					_, err := db.deprecatedInfoDB.GetDB().ExecContext(ctx, "VACUUM;")
					return err
				}),
			},
			{
				DB:          &db.deprecatedInfoDB.DB,
				Description: "Split into multiple sqlite databases",
				Version:     23,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, BandwidthDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.openDatabase(ctx, OrdersDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.openDatabase(ctx, PieceExpirationDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.openDatabase(ctx, PieceInfoDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.openDatabase(ctx, PieceSpaceUsedDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.openDatabase(ctx, ReputationDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.openDatabase(ctx, StorageUsageDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.openDatabase(ctx, UsedSerialsDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					if err := db.openDatabase(ctx, SatellitesDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}

					return nil
				},
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
				DB:          &db.deprecatedInfoDB.DB,
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
				DB:          &db.satellitesDB.DB,
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
				DB:          &db.pieceExpirationDB.DB,
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
				DB:          &db.ordersDB.DB,
				Description: "Add index archived_at to ordersDB",
				Version:     27,
				Action: migrate.SQL{
					`CREATE INDEX idx_order_archived_at ON order_archive_(archived_at)`,
				},
			},
			{
				DB:          &db.notificationsDB.DB,
				Description: "Create notifications table",
				Version:     28,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, NotificationsDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}

					return nil
				},
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
				DB:          &db.pieceSpaceUsedDB.DB,
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
				DB:          &db.pieceSpaceUsedDB.DB,
				Description: "Initialize piece_space_used total column to content_size",
				Version:     30,
				Action: migrate.SQL{
					`UPDATE piece_space_used SET total = content_size`,
				},
			},
			{
				DB:          &db.pieceSpaceUsedDB.DB,
				Description: "Remove all 0 values from piece_space_used",
				Version:     31,
				Action: migrate.SQL{
					`UPDATE piece_space_used SET total = 0 WHERE total < 0`,
					`UPDATE piece_space_used SET content_size = 0 WHERE content_size < 0`,
				},
			},
			{
				DB:          &db.payoutDB.DB,
				Description: "Create paystubs table and payments table",
				Version:     32,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, HeldAmountDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}

					return nil
				},
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
				DB:          &db.payoutDB.DB,
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
				DB:          &db.reputationDB.DB,
				Description: "Add suspended field to satellites db",
				Version:     34,
				Action: migrate.SQL{
					`ALTER TABLE reputation ADD COLUMN suspended TIMESTAMP`,
				},
			},
			{
				DB:          &db.pricingDB.DB,
				Description: "Create pricing table",
				Version:     35,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, PricingDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}

					return nil
				},
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
				DB:          &db.reputationDB.DB,
				Description: "Add joined_at field to satellites db",
				Version:     36,
				Action: migrate.SQL{
					`ALTER TABLE reputation ADD COLUMN joined_at TIMESTAMP`,
				},
			},
			{
				DB:          &db.payoutDB.DB,
				Description: "Drop payments table as unused",
				Version:     37,
				Action: migrate.SQL{
					`DROP TABLE payments;`,
				},
			},
			{
				DB:          &db.reputationDB.DB,
				Description: "Backfill joined_at column",
				Version:     38,
				Action: migrate.Func(func(ctx context.Context, _ *zap.Logger, rdb tagsql.DB, rtx tagsql.Tx) (err error) {
					// We just need a value for joined_at until the node checks in with the
					// satellites and gets the real value.
					_, err = rtx.ExecContext(ctx, `UPDATE reputation SET joined_at = ? WHERE joined_at ISNULL`, time.Unix(0, 0).UTC())
					if err != nil {
						return errs.Wrap(err)
					}

					// in order to add the not null constraint, we have to do a
					// generalized ALTER TABLE procedure.
					// see https://www.sqlite.org/lang_altertable.html
					_, err = rtx.ExecContext(ctx, `
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
				DB:          &db.reputationDB.DB,
				Description: "Add unknown_audit_reputation_alpha and unknown_audit_reputation_beta fields to reputation db",
				Version:     39,
				Action: migrate.Func(func(ctx context.Context, _ *zap.Logger, rdb tagsql.DB, rtx tagsql.Tx) (err error) {
					_, err = rtx.ExecContext(ctx, `ALTER TABLE reputation ADD COLUMN audit_unknown_reputation_alpha REAL`)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.ExecContext(ctx, `ALTER TABLE reputation ADD COLUMN audit_unknown_reputation_beta REAL`)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.ExecContext(ctx, `UPDATE reputation SET audit_unknown_reputation_alpha = ?, audit_unknown_reputation_beta = ?`,
						1.0, 1.0)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.ExecContext(ctx, `
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
			{
				DB:          &db.reputationDB.DB,
				Description: "Add unknown_audit_reputation_score field to reputation db",
				Version:     40,
				Action: migrate.Func(func(ctx context.Context, _ *zap.Logger, rdb tagsql.DB, rtx tagsql.Tx) (err error) {
					_, err = rtx.ExecContext(ctx, `ALTER TABLE reputation ADD COLUMN audit_unknown_reputation_score REAL`)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.ExecContext(ctx, `UPDATE reputation SET audit_unknown_reputation_score = ?`,
						1.0)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.ExecContext(ctx, `
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
							audit_unknown_reputation_score REAL NOT NULL,
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
							audit_unknown_reputation_score,
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
				DB:          &db.satellitesDB.DB,
				Description: "Make satellite_id foreign key in satellite_exit_progress table",
				Version:     41,
				Action: migrate.Func(func(ctx context.Context, _ *zap.Logger, rdb tagsql.DB, rtx tagsql.Tx) (err error) {
					_, err = rtx.ExecContext(ctx, `
						CREATE TABLE satellite_exit_progress_new (
							satellite_id BLOB NOT NULL,
							initiated_at TIMESTAMP,
							finished_at TIMESTAMP,
							starting_disk_usage INTEGER NOT NULL,
							bytes_deleted INTEGER NOT NULL,
							completion_receipt BLOB,
							FOREIGN KEY (satellite_id) REFERENCES satellites (node_id)
						);

						INSERT INTO satellite_exit_progress_new SELECT
							satellite_id,
							initiated_at,
							finished_at,
							starting_disk_usage,
							bytes_deleted,
							completion_receipt
						FROM satellite_exit_progress;

						DROP TABLE satellite_exit_progress;

						ALTER TABLE satellite_exit_progress_new RENAME TO satellite_exit_progress;
					`)
					if err != nil {
						return errs.Wrap(err)
					}

					return nil
				}),
			},
			{
				DB:          &db.usedSerialsDB.DB,
				Description: "Drop used serials table",
				Version:     42,
				Action: migrate.Func(func(ctx context.Context, _ *zap.Logger, rdb tagsql.DB, rtx tagsql.Tx) (err error) {
					_, err = rtx.ExecContext(ctx, `
						DROP TABLE used_serial_;
					`)
					if err != nil {
						return errs.Wrap(err)
					}

					return nil
				}),
			},
			{
				DB:          &db.payoutDB.DB,
				Description: "Add table payments",
				Version:     43,
				Action: migrate.SQL{
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
				DB:          &db.reputationDB.DB,
				Description: "Add online_score and offline_suspended fields to reputation db, rename disqualified and suspended to disqualified_at and suspended_at",
				Version:     44,
				Action: migrate.Func(func(ctx context.Context, _ *zap.Logger, rdb tagsql.DB, rtx tagsql.Tx) (err error) {
					_, err = rtx.ExecContext(ctx, `ALTER TABLE reputation ADD COLUMN online_score REAL`)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.ExecContext(ctx, `ALTER TABLE reputation ADD COLUMN offline_suspended_at TIMESTAMP`)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.ExecContext(ctx, `ALTER TABLE reputation RENAME COLUMN disqualified TO disqualified_at`)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.ExecContext(ctx, `ALTER TABLE reputation RENAME COLUMN suspended TO suspended_at`)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.ExecContext(ctx, `UPDATE reputation SET online_score = ?`,
						1.0)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.ExecContext(ctx, `
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
							audit_unknown_reputation_score REAL NOT NULL,
							online_score REAL NOT NULL,
							disqualified_at TIMESTAMP,
							updated_at TIMESTAMP NOT NULL,
							suspended_at TIMESTAMP,
							offline_suspended_at TIMESTAMP,
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
							audit_unknown_reputation_score,
							online_score,
							disqualified_at,
							updated_at,
							suspended_at,
							offline_suspended_at,
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
				DB:          &db.reputationDB.DB,
				Description: "Add offline_under_review_at field to reputation db",
				Version:     45,
				Action: migrate.Func(func(ctx context.Context, _ *zap.Logger, rdb tagsql.DB, rtx tagsql.Tx) (err error) {
					_, err = rtx.ExecContext(ctx, `ALTER TABLE reputation ADD COLUMN offline_under_review_at TIMESTAMP`)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.ExecContext(ctx, `
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
							audit_unknown_reputation_score REAL NOT NULL,
							online_score REAL NOT NULL,
							disqualified_at TIMESTAMP,
							updated_at TIMESTAMP NOT NULL,
							suspended_at TIMESTAMP,
							offline_suspended_at TIMESTAMP,
							offline_under_review_at TIMESTAMP,
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
							audit_unknown_reputation_score,
							online_score,
							disqualified_at,
							updated_at,
							suspended_at,
							offline_suspended_at,
							offline_under_review_at,
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
				DB:          &db.apiKeysDB.DB,
				Description: "Create secret table",
				Version:     46,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, APIKeysDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}

					return nil
				},
				Action: migrate.SQL{
					`CREATE TABLE secret (
						token bytea NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( token )
					);`,
				},
			},
			{
				DB:          &db.reputationDB.DB,
				Description: "Add audit_history field to reputation db",
				Version:     47,
				Action: migrate.SQL{
					`ALTER TABLE reputation ADD COLUMN audit_history BLOB`,
				},
			},
			{
				DB:          &db.reputationDB.DB,
				Description: "drop uptime columns",
				Version:     48,
				Action: migrate.Func(func(ctx context.Context, _ *zap.Logger, rdb tagsql.DB, rtx tagsql.Tx) (err error) {
					_, err = rtx.ExecContext(ctx, `
						CREATE TABLE reputation_new (
							satellite_id BLOB NOT NULL,
							audit_success_count INTEGER NOT NULL,
							audit_total_count INTEGER NOT NULL,
							audit_reputation_alpha REAL NOT NULL,
							audit_reputation_beta REAL NOT NULL,
							audit_reputation_score REAL NOT NULL,
							audit_unknown_reputation_alpha REAL NOT NULL,
							audit_unknown_reputation_beta REAL NOT NULL,
							audit_unknown_reputation_score REAL NOT NULL,
							online_score REAL NOT NULL,
							audit_history BLOB,
							disqualified_at TIMESTAMP,
							updated_at TIMESTAMP NOT NULL,
							suspended_at TIMESTAMP,
							offline_suspended_at TIMESTAMP,
							offline_under_review_at TIMESTAMP,
							joined_at TIMESTAMP NOT NULL,
							PRIMARY KEY (satellite_id)
						);
						INSERT INTO reputation_new SELECT
							satellite_id,
							audit_success_count,
							audit_total_count,
							audit_reputation_alpha,
							audit_reputation_beta,
							audit_reputation_score,
							audit_unknown_reputation_alpha,
							audit_unknown_reputation_beta,
							audit_unknown_reputation_score,
							online_score,
							audit_history,
							disqualified_at,
							updated_at,
							suspended_at,
							offline_suspended_at,
							offline_under_review_at,
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
				DB:          &db.payoutDB.DB,
				Description: "Add distributed field to paystubs table",
				Version:     49,
				Action: migrate.SQL{
					`ALTER TABLE paystubs ADD COLUMN distributed bigint`,
				},
			},
			{
				DB:          &db.payoutDB.DB,
				Description: "Make distributed field in paystubs table not null",
				Version:     50,
				Action: migrate.Func(func(ctx context.Context, _ *zap.Logger, rdb tagsql.DB, rtx tagsql.Tx) (err error) {
					_, err = rtx.ExecContext(ctx, `UPDATE paystubs SET distributed = ? WHERE distributed ISNULL`, 0)
					if err != nil {
						return errs.Wrap(err)
					}

					_, err = rtx.ExecContext(ctx, `
						CREATE TABLE paystubs_new (
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
							distributed bigint NOT NULL,
							PRIMARY KEY ( period, satellite_id )
						);
						INSERT INTO paystubs_new SELECT
							period,
							satellite_id,
							created_at,
							codes,
							usage_at_rest,
							usage_get,
							usage_put,
							usage_get_repair,
							usage_put_repair,
							usage_get_audit,
							comp_at_rest,
							comp_get,
							comp_put,
							comp_get_repair,
							comp_put_repair,
							comp_get_audit,
							surge_percent,
							held,
							owed,
							disposed,
							paid,
							distributed
							FROM paystubs;
						DROP TABLE paystubs;
						ALTER TABLE paystubs_new RENAME TO paystubs;
					`)
					if err != nil {
						return errs.Wrap(err)
					}

					return nil
				}),
			},
			{
				DB:          &db.payoutDB.DB,
				Description: "Assume distributed == paid for paystubs before 2020-12.",
				Version:     51,
				Action: migrate.SQL{
					`UPDATE paystubs SET distributed = paid WHERE period < '2020-12'`,
				},
			},
			{
				DB:          &db.reputationDB.DB,
				Description: "Add vetted_at field to reputation db",
				Version:     52,
				Action: migrate.SQL{
					`ALTER TABLE reputation ADD COLUMN vetted_at TIMESTAMP`,
				},
			},
			{
				DB:          &db.satellitesDB.DB,
				Description: "Add address to satellites, inserts stefan-benten satellite into satellites db",
				Version:     53,
				Action: migrate.SQL{
					`ALTER TABLE satellites ADD COLUMN address TEXT;
					 UPDATE satellites SET address = 'satellite.stefan-benten.de:7777' WHERE node_id = X'004ae89e970e703df42ba4ab1416a3b30b7e1d8e14aa0e558f7ee26800000000'`,
				},
			},
			{
				DB:          &db.storageUsageDB.DB,
				Description: "Add interval_end_time field to storage_usage db, backfill interval_end_time with interval_start, rename interval_start to timestamp",
				Version:     54,
				Action: migrate.Func(func(ctx context.Context, _ *zap.Logger, rdb tagsql.DB, rtx tagsql.Tx) error {
					_, err := rtx.ExecContext(ctx, `
						CREATE TABLE storage_usage_new (
							timestamp TIMESTAMP NOT NULL,
							satellite_id BLOB NOT NULL,
							at_rest_total REAL NOT NULL,
							interval_end_time TIMESTAMP NOT NULL,
							PRIMARY KEY (timestamp, satellite_id)
						);
						INSERT INTO storage_usage_new SELECT
							interval_start,
							satellite_id,
							at_rest_total,
							interval_start
						FROM storage_usage;
						DROP TABLE storage_usage;
						ALTER TABLE storage_usage_new RENAME TO storage_usage;
					`)

					return errs.Wrap(err)
				}),
			},
			{
				DB:          &db.gcFilewalkerProgressDB.DB,
				Description: "Create gc_filewalker_progress db",
				Version:     55,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, GCFilewalkerProgressDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}

					return nil
				},
				Action: migrate.SQL{
					`CREATE TABLE progress (
						satellite_id BLOB NOT NULL,
						bloomfilter_created_before TIMESTAMP NOT NULL,
						last_checked_prefix TEXT NOT NULL,
						PRIMARY KEY (satellite_id)
					);`,
				},
			},
			{
				DB:          &db.usedSpacePerPrefixDB.DB,
				Description: "Create used_space_per_prefix db",
				Version:     56,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, UsedSpacePerPrefixDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}

					return nil
				},
				Action: migrate.SQL{
					`CREATE TABLE used_space_per_prefix (
						satellite_id BLOB NOT NULL,
						piece_prefix TEXT NOT NULL,
						total_bytes INTEGER NOT NULL,
						last_updated TIMESTAMP NOT NULL,
						PRIMARY KEY (satellite_id, piece_prefix)
					);`,
				},
			},
			{
				DB:          &db.bandwidthDB.DB,
				Description: "Create new bandwidth_usage table, backfilling data from bandwidth_usage_rollups and bandwidth_usage tables, and dropping the old tables.",
				Version:     57,
				Action: migrate.SQL{`
						CREATE TABLE bandwidth_usage_new (
							interval_start   TIMESTAMP NOT NULL,
							satellite_id     BLOB      NOT NULL,
							put_total        BIGINT DEFAULT 0,
							get_total        BIGINT DEFAULT 0,
							get_audit_total  BIGINT DEFAULT 0,
							get_repair_total BIGINT DEFAULT 0,
							put_repair_total BIGINT DEFAULT 0,
							delete_total     BIGINT DEFAULT 0,
							PRIMARY KEY (interval_start, satellite_id)
						);

						INSERT INTO bandwidth_usage_new (
							interval_start,
							satellite_id,
							put_total,
							get_total,
							get_audit_total,
							get_repair_total,
							put_repair_total,
							delete_total
						)
						SELECT
							datetime(date(interval_start)) as interval_start,
							satellite_id,
							SUM(CASE WHEN action = 1 THEN amount ELSE 0 END) AS put_total,
							SUM(CASE WHEN action = 2 THEN amount ELSE 0 END) AS get_total,
							SUM(CASE WHEN action = 3 THEN amount ELSE 0 END) AS get_audit_total,
							SUM(CASE WHEN action = 4 THEN amount ELSE 0 END) AS get_repair_total,
							SUM(CASE WHEN action = 5 THEN amount ELSE 0 END) AS put_repair_total,
							SUM(CASE WHEN action = 6 THEN amount ELSE 0 END) AS delete_total
						FROM
						    bandwidth_usage_rollups
						WHERE -- protection against data corruption
							datetime(interval_start) IS NOT NULL AND
							satellite_id IS NOT NULL AND
							1 <= action AND action <= 6
						GROUP BY
							datetime(date(interval_start)), satellite_id, action
						ON CONFLICT(interval_start, satellite_id) DO UPDATE SET
							put_total        = put_total + excluded.put_total,
							get_total        = get_total + excluded.get_total,
							get_audit_total  = get_audit_total + excluded.get_audit_total,
							get_repair_total = get_repair_total + excluded.get_repair_total,
							put_repair_total = put_repair_total + excluded.put_repair_total,
							delete_total     = delete_total + excluded.delete_total;

						-- Backfill data from bandwidth_usage table
						INSERT INTO bandwidth_usage_new (
							interval_start,
							satellite_id,
							put_total,
							get_total,
							get_audit_total,
							get_repair_total,
							put_repair_total,
							delete_total
						)
						SELECT
							datetime(date(created_at)) as interval_start,
							satellite_id,
							SUM(CASE WHEN action = 1 THEN amount ELSE 0 END) AS put_total,
							SUM(CASE WHEN action = 2 THEN amount ELSE 0 END) AS get_total,
							SUM(CASE WHEN action = 3 THEN amount ELSE 0 END) AS get_audit_total,
							SUM(CASE WHEN action = 4 THEN amount ELSE 0 END) AS get_repair_total,
							SUM(CASE WHEN action = 5 THEN amount ELSE 0 END) AS put_repair_total,
							SUM(CASE WHEN action = 6 THEN amount ELSE 0 END) AS delete_total
						FROM
						    bandwidth_usage
						WHERE -- protection against data corruption
							datetime(created_at) IS NOT NULL AND
							satellite_id IS NOT NULL AND
							1 <= action AND action <= 6
						GROUP BY
							datetime(date(created_at)), satellite_id, action
						ON CONFLICT(interval_start, satellite_id) DO UPDATE SET
							put_total        = put_total + excluded.put_total,
							get_total        = get_total + excluded.get_total,
							get_audit_total  = get_audit_total + excluded.get_audit_total,
							get_repair_total = get_repair_total + excluded.get_repair_total,
							put_repair_total = put_repair_total + excluded.put_repair_total,
							delete_total     = delete_total + excluded.delete_total;

						DROP TABLE bandwidth_usage_rollups;
						DROP TABLE bandwidth_usage;
						ALTER TABLE bandwidth_usage_new RENAME TO bandwidth_usage;
					`,
				},
			},
			{
				DB:          &db.pieceExpirationDB.DB,
				Description: "Remove unused trash column",
				Version:     58,
				Action: migrate.SQL{
					`DROP INDEX idx_piece_expirations_trashed;`,
					`ALTER TABLE piece_expirations DROP COLUMN trash;`,
				},
			},
			{
				DB:          &db.pieceExpirationDB.DB,
				Description: "Remove unused deletion_failed_at column",
				Version:     59,
				Action: migrate.SQL{
					`DROP INDEX idx_piece_expirations_deletion_failed_at;`,
					`ALTER TABLE piece_expirations DROP COLUMN deletion_failed_at;`,
				},
			},
			{
				DB:          &db.pieceExpirationDB.DB,
				Description: "Overhaul piece_expirations",
				Version:     60,
				Action: migrate.SQL{
					`CREATE TABLE piece_expirations_new (
						satellite_id     BLOB      NOT NULL,
						piece_id         BLOB      NOT NULL,
						piece_expiration TIMESTAMP NOT NULL  -- date when it can be deleted
					);`,
					`INSERT INTO piece_expirations_new (satellite_id, piece_id, piece_expiration) SELECT satellite_id, piece_id, piece_expiration FROM piece_expirations;`,
					`DROP TABLE piece_expirations;`,
					`ALTER TABLE piece_expirations_new RENAME TO piece_expirations;`,
					`CREATE INDEX idx_piece_expirations_piece_expiration ON piece_expirations(piece_expiration);`,
				},
			},
			{
				DB:          &db.pieceSpaceUsedDB.DB,
				Description: "Remove records with null satellite ID values from piece_space_used table",
				Version:     61,
				Action: migrate.SQL{
					`DELETE FROM piece_space_used WHERE satellite_id IS NULL;`,
				},
			},
			{
				DB:          &db.usedSpacePerPrefixDB.DB,
				Description: "Add total_content_size, piece_counts, resume_point columns to used_space_per_prefix table",
				Version:     62,
				Action: migrate.SQL{
					`ALTER TABLE used_space_per_prefix ADD COLUMN total_content_size INTEGER NOT NULL DEFAULT 0`,
					`ALTER TABLE used_space_per_prefix ADD COLUMN piece_counts INTEGER NOT NULL DEFAULT 0`,
				},
			},
		},
	}
}
