// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/migrate"
)

var ErrInfo = errs.Class("infodb")

type infodb struct {
	mu sync.Mutex
	db *sql.DB
}

func NewInfo(path string) (*infodb, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", "file:"+path+"?_journal=WAL")
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	return &infodb{db: db}, nil
}

func NewInfoInMemory() (*infodb, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	return &infodb{db: db}, nil
}

// Close closes any resources.
func (db *infodb) Close() error {
	return db.db.Close()
}

func (db *infodb) locked() func() {
	db.mu.Lock()
	return db.mu.Unlock
}

// CreateTables creates any necessary tables.
func (db *infodb) CreateTables(log *zap.Logger) error {
	migration := db.Migration()
	return migration.Run(log.Named("migration"), db)
}

// RawDB returns access to the raw database, only for migration tests.
func (db *infodb) RawDB() *sql.DB { return db.db }

// Begin begins transaction
func (db *infodb) Begin() (*sql.Tx, error) { return db.db.Begin() }

// Rebind rebind parameters
func (db *infodb) Rebind(s string) string { return s }

// Schema returns schema
func (db *infodb) Schema() string { return "" }

// Migration returns table migrations.
func (db *infodb) Migration() *migrate.Migration {
	return &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				Description: "Initial setup",
				Version:     0,
				Action: migrate.SQL{
					// table for keeping serials that need to be verified against
					`CREATE TABLE used_serial (
						satellite_id  BLOB,
						serial_number BLOB,
						expiration    TIMESTAMP without time zone
					)`,
					// primary key on satellite id and serial number
					`CREATE UNIQUE INDEX pk_used_serial ON used_serial(satellite_id, serial_number)`,
					// expiration index to allow fast deletion
					`CREATE INDEX idx_used_serial ON used_serial(expiration)`,

					// certificate table for storing uplink/satellite certificates
					`CREATE TABLE certificate (
						cert_id  INTEGER PRIMARY KEY AUTOINCREMENT,
						node_id  BLOB,
						pkix     BLOB UNIQUE
					)`,

					// table for storing piece meta info
					`CREATE TABLE pieceinfo (
						satellite_id     BLOB,
						piece_id         BLOB,
						piece_size       BIGINT,
						piece_expiration TIMESTAMP without time zone, -- date when it can be deleted

						uplink_hash   BLOB, -- serialized pb.PieceHash signed by uplink
						uplink_cert_id INTEGER,

						FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
					)`,

					// primary key by satellite id and piece id
					`CREATE UNIQUE INDEX pk_pieceinfo ON pieceinfo(satellite_id, piece_id)`,

					// table for storing bandwidth usage
					`CREATE TABLE bandwidth_usage (
						satellite_id  BLOB,
						action        INTEGER,
						amount        BIGINT,
						created_at    TIMESTAMP without time zone
					)`,
					`CREATE INDEX idx_bandwidth_usage_satellite ON bandwidth_usage(satellite_id)`,
					`CREATE INDEX idx_bandwidth_usage_created   ON bandwidth_usage(created_at)`,

					// table for storing all unsent orders
					`CREATE TABLE unsent_order (
						satellite_id  BLOB,
						serial_number BLOB,

						order_limit_serialized BLOB, -- serialized pb.OrderLimit
						order_serialized       BLOB, -- serialized pb.Order
						order_limit_expiration TIMESTAMP without time zone, -- when is the deadline for sending it

						uplink_cert_id INTEGER,

						FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
					)`,
					`CREATE INDEX idx_orders ON unsent_order(satellite_id, serial_number)`,

					// table for storing all sent orders
					`CREATE TABLE order_archive (
						satellite_id  BLOB,
						serial_number BLOB,
						
						order_limit_serialized BLOB, -- serialized pb.OrderLimit
						order_serialized       BLOB, -- serialized pb.Order
						
						uplink_cert_id INTEGER,
						
						status INT, -- accepted, rejected, confirmed
						archived_at TIMESTAMP without time zone, -- when was it rejected
						
						FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
					)`,
					`CREATE INDEX idx_order_archive_satellite ON order_archive(satellite_id)`,
					`CREATE INDEX idx_order_archive_status ON order_archive(status)`,
				},
			},
		},
	}
}
