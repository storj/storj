// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package metabase implements storing objects and segements.
package metabase

import (
	"context"
	"strconv"

	_ "github.com/jackc/pgx/v4"        // registers pgx as a tagsql driver.
	_ "github.com/jackc/pgx/v4/stdlib" // registers pgx as a tagsql driver.
	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/migrate"
	"storj.io/storj/private/tagsql"
)

var (
	mon = monkit.Package()
)

// DB implements a database for storing objects and segments.
type DB struct {
	log *zap.Logger
	db  tagsql.DB
}

// Open opens a connection to metabase.
func Open(ctx context.Context, log *zap.Logger, driverName, connstr string) (*DB, error) {
	db, err := tagsql.Open(ctx, driverName, connstr)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	dbutil.Configure(ctx, db, "metabase", mon)

	return &DB{log: log, db: postgresRebind{db}}, nil
}

// InternalImplementation returns *metabase.DB
// TODO: remove.
func (db *DB) InternalImplementation() interface{} { return db }

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
	migration := db.PostgresMigration()
	return migration.Run(ctx, db.log.Named("migrate"))
}

// PostgresMigration returns steps needed for migrating postgres database.
func (db *DB) PostgresMigration() *migrate.Migration {
	// TODO: merge this with satellite migration code or a way to keep them in sync.
	return &migrate.Migration{
		Table: "metabase_versions",
		Steps: []*migrate.Step{
			{
				DB:          &db.db,
				Description: "initial setup",
				Version:     1,
				Action: migrate.SQL{
					`CREATE TABLE objects (
						project_id   BYTEA NOT NULL,
						bucket_name  BYTEA NOT NULL, -- we're using bucket_name here to avoid a lookup into buckets table
						object_key   BYTEA NOT NULL, -- using 'object_key' instead of 'key' to avoid reserved word
						version      INT4  NOT NULL,
						stream_id    BYTEA NOT NULL,

						created_at TIMESTAMPTZ NOT NULL default now(),
						expires_at TIMESTAMPTZ,

						status         INT2 NOT NULL default ` + pendingStatus + `,
						segment_count  INT4 NOT NULL default 0,

						encrypted_metadata_nonce         BYTEA default NULL,
						encrypted_metadata               BYTEA default NULL,
						encrypted_metadata_encrypted_key BYTEA default NULL,

						total_encrypted_size INT4 NOT NULL default 0,
						fixed_segment_size   INT4 NOT NULL default 0,

						encryption INT8 NOT NULL default 0,

						zombie_deletion_deadline TIMESTAMPTZ default now() + '1 day', -- should this be in a separate table?

						PRIMARY KEY (project_id, bucket_name, object_key, version)
					)`,
					`CREATE TABLE segments (
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
					)`,
				},
			},
		},
	}
}

// This is needed for migrate to work.
// TODO: clean this up.
type postgresRebind struct{ tagsql.DB }

func (pq postgresRebind) Rebind(sql string) string {
	type sqlParseState int
	const (
		sqlParseStart sqlParseState = iota
		sqlParseInStringLiteral
		sqlParseInQuotedIdentifier
		sqlParseInComment
	)

	out := make([]byte, 0, len(sql)+10)

	j := 1
	state := sqlParseStart
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		switch state {
		case sqlParseStart:
			switch ch {
			case '?':
				out = append(out, '$')
				out = append(out, strconv.Itoa(j)...)
				state = sqlParseStart
				j++
				continue
			case '-':
				if i+1 < len(sql) && sql[i+1] == '-' {
					state = sqlParseInComment
				}
			case '"':
				state = sqlParseInQuotedIdentifier
			case '\'':
				state = sqlParseInStringLiteral
			}
		case sqlParseInStringLiteral:
			if ch == '\'' {
				state = sqlParseStart
			}
		case sqlParseInQuotedIdentifier:
			if ch == '"' {
				state = sqlParseStart
			}
		case sqlParseInComment:
			if ch == '\n' {
				state = sqlParseStart
			}
		}
		out = append(out, ch)
	}

	return string(out)
}
