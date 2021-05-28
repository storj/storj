// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package metabase implements storing objects and segements.
package metabase

import (
	"context"
	"fmt"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v4"        // registers pgx as a tagsql driver.
	_ "github.com/jackc/pgx/v4/stdlib" // registers pgx as a tagsql driver.
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/dbutil"
	"storj.io/private/dbutil/pgutil"
	"storj.io/private/tagsql"
	"storj.io/storj/private/migrate"
)

var (
	mon = monkit.Package()
)

// DB implements a database for storing objects and segments.
type DB struct {
	log     *zap.Logger
	db      tagsql.DB
	connstr string
	impl    dbutil.Implementation

	aliasCache *NodeAliasCache

	testCleanup func() error
}

// Open opens a connection to metabase.
func Open(ctx context.Context, log *zap.Logger, connstr string) (*DB, error) {
	var driverName string
	_, _, impl, err := dbutil.SplitConnStr(connstr)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	switch impl {
	case dbutil.Postgres:
		driverName = "pgx"
	case dbutil.Cockroach:
		driverName = "cockroach"
	default:
		return nil, Error.New("unsupported implementation: %s", connstr)
	}

	rawdb, err := tagsql.Open(ctx, driverName, connstr)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	dbutil.Configure(ctx, rawdb, "metabase", mon)

	db := &DB{
		log:         log,
		db:          postgresRebind{rawdb},
		connstr:     connstr,
		impl:        impl,
		testCleanup: func() error { return nil },
	}
	db.aliasCache = NewNodeAliasCache(db)

	log.Debug("Connected", zap.String("db source", connstr))

	return db, nil
}

// InternalImplementation returns *metabase.DB.
// TODO: remove.
func (db *DB) InternalImplementation() interface{} { return db }

// UnderlyingTagSQL returns *tagsql.DB.
// TODO: remove.
func (db *DB) UnderlyingTagSQL() tagsql.DB { return db.db }

// Ping checks whether connection has been established.
func (db *DB) Ping(ctx context.Context) error {
	return Error.Wrap(db.db.PingContext(ctx))
}

// TestingSetCleanup is used to set the callback for cleaning up test database.
func (db *DB) TestingSetCleanup(cleanup func() error) {
	db.testCleanup = cleanup
}

// Close closes the connection to database.
func (db *DB) Close() error {
	return errs.Combine(Error.Wrap(db.db.Close()), db.testCleanup())
}

// DestroyTables deletes all tables.
//
// TODO: remove this, only for bootstrapping.
func (db *DB) DestroyTables(ctx context.Context) error {
	_, err := db.db.ExecContext(ctx, `
		DROP TABLE IF EXISTS objects;
		DROP TABLE IF EXISTS segments;
		DROP TABLE IF EXISTS node_aliases;
		DROP SEQUENCE IF EXISTS node_alias_seq;
	`)
	db.aliasCache = NewNodeAliasCache(db)
	return Error.Wrap(err)
}

// MigrateToLatest migrates database to the latest version.
func (db *DB) MigrateToLatest(ctx context.Context) error {
	// First handle the idiosyncrasies of postgres and cockroach migrations. Postgres
	// will need to create any schemas specified in the search path, and cockroach
	// will need to create the database it was told to connect to. These things should
	// not really be here, and instead should be assumed to exist.
	// This is tracked in jira ticket SM-200
	switch db.impl {
	case dbutil.Postgres:
		schema, err := pgutil.ParseSchemaFromConnstr(db.connstr)
		if err != nil {
			return errs.New("error parsing schema: %+v", err)
		}

		if schema != "" {
			err = pgutil.CreateSchema(ctx, db.db, schema)
			if err != nil {
				return errs.New("error creating schema: %+v", err)
			}
		}

	case dbutil.Cockroach:
		var dbName string
		if err := db.db.QueryRow(ctx, `SELECT current_database();`).Scan(&dbName); err != nil {
			return errs.New("error querying current database: %+v", err)
		}

		_, err := db.db.Exec(ctx, fmt.Sprintf(`CREATE DATABASE IF NOT EXISTS %s;`,
			pgutil.QuoteIdentifier(dbName)))
		if err != nil {
			return errs.Wrap(err)
		}
	}

	migration := db.PostgresMigration()
	return migration.Run(ctx, db.log.Named("migrate"))
}

// CheckVersion checks the database is the correct version.
func (db *DB) CheckVersion(ctx context.Context) error {
	migration := db.PostgresMigration()
	return migration.ValidateVersions(ctx, db.log)
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

						total_plain_size     INT4 NOT NULL default 0, -- migrated objects have this = 0
						total_encrypted_size INT4 NOT NULL default 0,
						fixed_segment_size   INT4 NOT NULL default 0, -- migrated objects have this = 0

						encryption INT8 NOT NULL default 0,

						zombie_deletion_deadline TIMESTAMPTZ default now() + '1 day',

						PRIMARY KEY (project_id, bucket_name, object_key, version)
					)`,
					`CREATE TABLE segments (
						stream_id  BYTEA NOT NULL,
						position   INT8  NOT NULL,

						root_piece_id       BYTEA NOT NULL,
						encrypted_key_nonce BYTEA NOT NULL,
						encrypted_key       BYTEA NOT NULL,

						encrypted_size INT4 NOT NULL,
						plain_offset   INT8 NOT NULL, -- migrated objects have this = 0
						plain_size     INT4 NOT NULL, -- migrated objects have this = 0

						redundancy INT8 NOT NULL default 0,

						inline_data  BYTEA DEFAULT NULL,
						remote_pieces BYTEA[],

						PRIMARY KEY (stream_id, position)
					)`,
				},
			},
			{
				DB:          &db.db,
				Description: "change total_plain_size and total_encrypted_size to INT8",
				Version:     2,
				Action: migrate.SQL{
					`ALTER TABLE objects ALTER COLUMN total_plain_size TYPE INT8;`,
					`ALTER TABLE objects ALTER COLUMN total_encrypted_size TYPE INT8;`,
				},
			},
			{
				DB:          &db.db,
				Description: "add node aliases table",
				Version:     3,
				Action: migrate.SQL{
					// We use a custom sequence to ensure small alias values.
					`CREATE SEQUENCE node_alias_seq
						INCREMENT BY 1
						MINVALUE 1 MAXVALUE 2147483647 -- MaxInt32
						START WITH 1
					`,
					`CREATE TABLE node_aliases (
						node_id    BYTEA  NOT NULL UNIQUE,
						node_alias INT4   NOT NULL UNIQUE default nextval('node_alias_seq')
					)`,
				},
			},
			{
				DB:          &db.db,
				Description: "add remote_alias_pieces column",
				Version:     4,
				Action: migrate.SQL{
					`ALTER TABLE segments ADD COLUMN remote_alias_pieces BYTEA`,
				},
			},
			{
				DB:          &db.db,
				Description: "drop remote_pieces from segments table",
				Version:     6,
				Action: migrate.SQL{
					`ALTER TABLE segments DROP COLUMN remote_pieces`,
				},
			},
			{
				DB:          &db.db,
				Description: "add created_at and repaired_at columns to segments table",
				Version:     7,
				Action: migrate.SQL{
					`ALTER TABLE segments ADD COLUMN created_at TIMESTAMPTZ`,
					`ALTER TABLE segments ADD COLUMN repaired_at TIMESTAMPTZ`,
				},
			},
			{
				DB:          &db.db,
				Description: "change default of created_at column in segments table to now()",
				Version:     8,
				Action: migrate.SQL{
					`ALTER TABLE segments ALTER COLUMN created_at SET DEFAULT now()`,
				},
			},
			{
				DB:          &db.db,
				Description: "add encrypted_etag column to segments table",
				Version:     9,
				Action: migrate.SQL{
					`ALTER TABLE segments ADD COLUMN encrypted_etag BYTEA default NULL`,
				},
			},
			{
				DB:          &db.db,
				Description: "add index on pending objects",
				Version:     10,
				Action:      migrate.SQL{},
			},
			{
				DB:          &db.db,
				Description: "drop pending_index on objects",
				Version:     11,
				Action: migrate.SQL{
					`DROP INDEX IF EXISTS pending_index`,
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

// Now returns time on the database.
func (db *DB) Now(ctx context.Context) (time.Time, error) {
	var t time.Time
	err := db.db.QueryRowContext(ctx, `SELECT now()`).Scan(&t)
	return t, Error.Wrap(err)
}
