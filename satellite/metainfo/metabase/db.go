// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package metabase implements storing objects and segements.
package metabase

import (
	"context"

	_ "github.com/jackc/pgx/v4"        // registers pgx as a tagsql driver.
	_ "github.com/jackc/pgx/v4/stdlib" // registers pgx as a tagsql driver.
	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/storj/private/tagsql"
)

var (
	mon = monkit.Package()
)

// DB implements a database for storing objects and segments.
type DB struct {
	db tagsql.DB
}

// Open opens a connection to metabase.
func Open(driverName, connstr string) (*DB, error) {
	db, err := tagsql.Open(driverName, connstr)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &DB{db: db}, nil
}

// Ping checks whether connection has been established.
func (db *DB) Ping(ctx context.Context) error {
	return Error.Wrap(db.db.PingContext(ctx))
}

// Close closes the connection to database.
func (db *DB) Close() error {
	return Error.Wrap(db.db.Close())
}

// DestroyTables deletes all tables.
//
// TODO: remove this, only for bootstrapping.
func (db *DB) DestroyTables(ctx context.Context) error {
	_, err := db.db.ExecContext(ctx, `
		DROP TABLE IF EXISTS objects;
		DROP TABLE IF EXISTS segments;
	`)
	return Error.Wrap(err)
}

// MigrateToLatest migrates database to the latest version.
//
// TODO: use migrate package.
func (db *DB) MigrateToLatest(ctx context.Context) error {
	var err error

	// TODO: verify whether this is all we need.
	_, err = db.db.ExecContext(ctx, `
		CREATE TABLE objects (
			project_id   BYTEA NOT NULL,
			bucket_name  BYTEA NOT NULL, -- we're using bucket_name here to avoid a lookup into buckets table
			object_key   BYTEA NOT NULL, -- using 'object_key' instead of 'key' to avoid reserved word
			version      INT4  NOT NULL,
			stream_id    BYTEA NOT NULL,

			created_at TIMESTAMPTZ NOT NULL default now(),
			expires_at TIMESTAMPTZ,

			status         INT2 NOT NULL default 0,
			segment_count  INT4 NOT NULL default 0,

			encrypted_metadata_nonce BYTEA default NULL,
			encrypted_metadata       BYTEA default NULL,

			total_encrypted_size INT4 NOT NULL default 0,
			fixed_segment_size   INT4 NOT NULL default 0,

			encryption INT8 NOT NULL default 0,

			zombie_deletion_deadline TIMESTAMPTZ default now() + '1 day', -- should this be in a separate table?

			PRIMARY KEY (project_id, bucket_name, object_key, version)
		);
	`)
	if err != nil {
		return Error.New("failed to create objects table: %w", err)
	}

	// TODO: verify whether this is all we need.
	_, err = db.db.ExecContext(ctx, `
		CREATE TABLE segments (
			stream_id  BYTEA NOT NULL,
			position   INT8  NOT NULL,

			root_piece_id       BYTEA NOT NULL,
			encrypted_key_nonce BYTEA NOT NULL,
			encrypted_key       BYTEA NOT NULL,

			encrypted_size INT4 NOT NULL, -- maybe this can be skipped?
			plain_offset   INT8 NOT NULL, -- this is needed to find segment based on plain byte offset
			plain_size     INT4 NOT NULL,

			redundancy INT8 NOT NULL default 0,

			inline_data  BYTEA DEFAULT NULL,
			remote_pieces BYTEA[],

			PRIMARY KEY (stream_id, position) -- TODO: should this use plain_offset for the primary index?
		)
	`)
	if err != nil {
		return Error.New("failed to create segments table: %w", err)
	}

	return nil
}
