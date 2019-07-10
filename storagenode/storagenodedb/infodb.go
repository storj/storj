// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/dbutil"
	"storj.io/storj/internal/migrate"
)

// ErrInfo is the default error class for InfoDB
var ErrInfo = errs.Class("infodb")

// InfoDB implements information database for piecestore.
type InfoDB struct {
	db          *sql.DB
	bandwidthdb bandwidthdb
	pieceinfo   pieceinfo
}

// newInfo creates or opens InfoDB at the specified path.
func newInfo(path string) (*InfoDB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", "file:"+path+"?_journal=WAL&_busy_timeout=10000")
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	dbutil.Configure(db, mon)

	infoDb := &InfoDB{db: db}
	infoDb.pieceinfo = pieceinfo{InfoDB: infoDb, space: spaceUsed{used: 0, once: sync.Once{}}}
	infoDb.bandwidthdb = bandwidthdb{InfoDB: infoDb, bandwidth: bandwidthUsed{used: 0, mu: sync.RWMutex{}, usedSince: time.Time{}}}

	return infoDb, nil
}

// NewInfoInMemory creates a new inmemory InfoDB.
func NewInfoInMemory() (*InfoDB, error) {
	// create memory DB with a shared cache and a unique name to avoid collisions
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:memdb%d?mode=memory&cache=shared", rand.Int63()))
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	// Set max idle and max open to 1 to control concurrent access to the memory DB
	// Setting max open higher than 1 results in table locked errors
	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(-1)

	mon.Chain("db_stats", monkit.StatSourceFunc(
		func(cb func(name string, val float64)) {
			monkit.StatSourceFromStruct(db.Stats()).Stats(cb)
		}))

	infoDb := &InfoDB{db: db}
	infoDb.pieceinfo = pieceinfo{InfoDB: infoDb, space: spaceUsed{used: 0, once: sync.Once{}}}
	infoDb.bandwidthdb = bandwidthdb{InfoDB: infoDb, bandwidth: bandwidthUsed{used: 0, mu: sync.RWMutex{}, usedSince: time.Time{}}}

	return infoDb, nil
}

// Close closes any resources.
func (db *InfoDB) Close() error {
	return db.db.Close()
}

// CreateTables creates any necessary tables.
func (db *InfoDB) CreateTables(log *zap.Logger) error {
	migration := db.Migration()
	return migration.Run(log.Named("migration"), db)
}

// RawDB returns access to the raw database, only for migration tests.
func (db *InfoDB) RawDB() *sql.DB { return db.db }

// Begin begins transaction
func (db *InfoDB) Begin() (*sql.Tx, error) { return db.db.Begin() }

// Rebind rebind parameters
func (db *InfoDB) Rebind(s string) string { return s }

// Schema returns schema
func (db *InfoDB) Schema() string { return "" }

// Migration returns table migrations.
func (db *InfoDB) Migration() *migrate.Migration {
	return &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
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
				Description: "Network Wipe #2",
				Version:     1,
				Action: migrate.SQL{
					`UPDATE pieceinfo SET piece_expiration = '2019-05-09 00:00:00.000000+00:00'`,
				},
			},
			{
				Description: "Add tracking of deletion failures.",
				Version:     2,
				Action: migrate.SQL{
					`ALTER TABLE pieceinfo ADD COLUMN deletion_failed_at TIMESTAMP`,
				},
			},
			{
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
				Description: "Add index on pieceinfo expireation",
				Version:     4,
				Action: migrate.SQL{
					`CREATE INDEX idx_pieceinfo_expiration ON pieceinfo(piece_expiration)`,
					`CREATE INDEX idx_pieceinfo_deletion_failed ON pieceinfo(deletion_failed_at)`,
				},
			},
			{
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
				Description: "Add creation date.",
				Version:     6,
				Action: migrate.SQL{
					`ALTER TABLE pieceinfo ADD COLUMN piece_creation TIMESTAMP NOT NULL DEFAULT 'epoch'`,
				},
			},
			{
				Description: "Drop certificate table.",
				Version:     7,
				Action: migrate.SQL{
					`DROP TABLE certificate`,
					`CREATE TABLE certificate (cert_id INTEGER)`,
				},
			},
			{
				Description: "Drop old used serials and remove pieceinfo_deletion_failed index.",
				Version:     8,
				Action: migrate.SQL{
					`DELETE FROM used_serial`,
					`DROP INDEX idx_pieceinfo_deletion_failed`,
				},
			},
		},
	}
}
