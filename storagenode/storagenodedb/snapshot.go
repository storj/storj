// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/private/migrate"
)

// Snapshot supposed to generate the same database as the Migration but without the original steps.
func (db *DB) Snapshot(ctx context.Context) *migrate.Migration {
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
				Action: migrate.SQL{},
			},
			{
				DB:          &db.bandwidthDB.DB,
				Description: "bandwidth db snapshot",
				Version:     2,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, BandwidthDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					return nil
				},
				Action: migrate.SQL{
					// table for storing bandwidth usage
					`CREATE TABLE bandwidth_usage (
						satellite_id  BLOB    NOT NULL,
						action        INTEGER NOT NULL,
						amount        BIGINT  NOT NULL,
						created_at    TIMESTAMP NOT NULL
					)`,
					`CREATE INDEX idx_bandwidth_usage_satellite ON bandwidth_usage(satellite_id)`,
					`CREATE INDEX idx_bandwidth_usage_created   ON bandwidth_usage(created_at)`,
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
				DB:          &db.ordersDB.DB,
				Description: "orders db snapshot",
				Version:     3,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, OrdersDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					return nil
				},
				Action: migrate.SQL{
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
					`CREATE INDEX idx_order_archived_at ON order_archive_(archived_at)`,
				},
			},
			{
				DB:          &db.pieceExpirationDB.DB,
				Description: "pieceExpiration db snapshot",
				Version:     4,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, PieceExpirationDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					return nil
				},
				Action: migrate.SQL{
					`CREATE TABLE piece_expirations (
						satellite_id       BLOB      NOT NULL,
						piece_id           BLOB      NOT NULL,
						piece_expiration   TIMESTAMP NOT NULL, -- date when it can be deleted
						deletion_failed_at TIMESTAMP, trash INTEGER NOT NULL DEFAULT 0,
						PRIMARY KEY (satellite_id, piece_id)
					)`,
					`CREATE INDEX idx_piece_expirations_piece_expiration ON piece_expirations(piece_expiration)`,
					`CREATE INDEX idx_piece_expirations_deletion_failed_at ON piece_expirations(deletion_failed_at)`,
					`CREATE INDEX idx_piece_expirations_trashed
						ON piece_expirations(satellite_id, trash)
						WHERE trash = 1`,
				},
			},
			{
				DB:          &db.v0PieceInfoDB.DB,
				Description: "v0PieceInfo db snapshot",
				Version:     5,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, PieceInfoDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					return nil
				},
				Action: migrate.SQL{
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
				},
			},
			{
				DB:          &db.pieceSpaceUsedDB.DB,
				Description: "pieceSpaceUsed db snapshot",
				Version:     6,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, PieceSpaceUsedDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					return nil
				},
				Action: migrate.SQL{
					// new table to hold the most recent totals from the piece space used cache
					`CREATE TABLE piece_space_used_new (
						total INTEGER NOT NULL DEFAULT 0,
						content_size INTEGER NOT NULL,
						satellite_id BLOB
					)`,
					`ALTER TABLE piece_space_used_new RENAME TO piece_space_used;`,
					`CREATE UNIQUE INDEX idx_piece_space_used_satellite_id ON piece_space_used(satellite_id)`,
				},
			},
			{
				DB:          &db.reputationDB.DB,
				Description: "reputation db snapshot",
				Version:     7,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, ReputationDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					return nil
				},
				Action: migrate.SQL{
					`	CREATE TABLE reputation_new (
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
							joined_at TIMESTAMP NOT NULL, vetted_at TIMESTAMP,
							PRIMARY KEY (satellite_id)
						);`,
					`ALTER TABLE reputation_new RENAME TO reputation;`,
				},
			},
			{
				DB:          &db.storageUsageDB.DB,
				Description: "storageUsage db snapshot",
				Version:     8,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, StorageUsageDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					return nil
				},
				Action: migrate.SQL{
					`CREATE TABLE storage_usage_new (
							timestamp TIMESTAMP NOT NULL,
							satellite_id BLOB NOT NULL,
							at_rest_total REAL NOT NULL,
							interval_end_time TIMESTAMP NOT NULL,
							PRIMARY KEY (timestamp, satellite_id)
						);`,
					`ALTER TABLE storage_usage_new RENAME TO storage_usage`,
				},
			},
			{
				DB:          &db.usedSerialsDB.DB,
				Description: "usedSerials db snapshot",
				Version:     9,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, UsedSerialsDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					return nil
				},
				Action: migrate.SQL{},
			},
			{
				DB:          &db.satellitesDB.DB,
				Description: "satellites db snapshot",
				Version:     10,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, SatellitesDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}
					return nil
				},
				Action: migrate.SQL{
					`CREATE TABLE satellites_new (
						node_id BLOB NOT NULL,
						added_at TIMESTAMP NOT NULL,
						status INTEGER NOT NULL, address TEXT,
						PRIMARY KEY (node_id)
					);`,
					`ALTER TABLE satellites_new RENAME TO satellites;`,
					`CREATE TABLE satellite_exit_progress_new (
							satellite_id BLOB NOT NULL,
							initiated_at TIMESTAMP,
							finished_at TIMESTAMP,
							starting_disk_usage INTEGER NOT NULL,
							bytes_deleted INTEGER NOT NULL,
							completion_receipt BLOB,
							FOREIGN KEY (satellite_id) REFERENCES satellites (node_id)
						);`,
					`ALTER TABLE satellite_exit_progress_new RENAME TO satellite_exit_progress`,
				},
			},

			{
				DB:          &db.notificationsDB.DB,
				Description: "notifications db snapshot",
				Version:     11,
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
				DB:          &db.payoutDB.DB,
				Description: "paystubs db snapshot",
				Version:     12,
				CreateDB: func(ctx context.Context, log *zap.Logger) error {
					if err := db.openDatabase(ctx, HeldAmountDBName); err != nil {
						return ErrDatabase.Wrap(err)
					}

					return nil
				},
				Action: migrate.SQL{
					`CREATE TABLE paystubs_new (
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
					`ALTER TABLE paystubs_new RENAME TO paystubs`,
				},
			},

			{
				DB:          &db.pricingDB.DB,
				Description: "pricing db snapshot",
				Version:     13,
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
				DB:          &db.apiKeysDB.DB,
				Description: "scret db snapshot",
				Version:     14,
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
		},
	}
}
