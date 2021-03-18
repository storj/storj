// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package metabase implements storing objects and segements.
package metabase

import (
	"context"
	"sort"
	"strconv"

	_ "github.com/jackc/pgx/v4"        // registers pgx as a tagsql driver.
	_ "github.com/jackc/pgx/v4/stdlib" // registers pgx as a tagsql driver.
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/migrate"
	"storj.io/storj/private/tagsql"
)

var (
	mon = monkit.Package()
)

// DB implements a database for storing objects and segments.
type DB struct {
	log            *zap.Logger
	db             tagsql.DB
	implementation dbutil.Implementation

	aliasCache *NodeAliasCache
}

// Open opens a connection to metabase.
func Open(ctx context.Context, log *zap.Logger, driverName, connstr string) (*DB, error) {
	rawdb, err := tagsql.Open(ctx, driverName, connstr)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	dbutil.Configure(ctx, rawdb, "metabase", mon)

	db := &DB{log: log, db: postgresRebind{rawdb}}
	db.aliasCache = NewNodeAliasCache(db)

	_, _, db.implementation, err = dbutil.SplitConnStr(connstr)
	if err != nil {
		return nil, Error.Wrap(err)
	}

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
		DROP TABLE IF EXISTS node_aliases;
		DROP SEQUENCE IF EXISTS node_alias_seq;
	`)
	db.aliasCache = NewNodeAliasCache(db)
	return Error.Wrap(err)
}

// MigrateToLatest migrates database to the latest version.
func (db *DB) MigrateToLatest(ctx context.Context) error {
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

						total_plain_size     INT4 NOT NULL default 0,
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
				Description: "convert remote_pieces to remote_alias_pieces",
				Version:     5,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, db tagsql.DB, tx tagsql.Tx) error {
					type segmentPieces struct {
						StreamID     uuid.UUID
						Position     SegmentPosition
						RemotePieces Pieces
					}

					var allSegments []segmentPieces

					err := withRows(tx.QueryContext(ctx, `SELECT stream_id, position, remote_pieces FROM segments WHERE remote_pieces IS NOT NULL`))(
						func(rows tagsql.Rows) error {
							for rows.Next() {
								var seg segmentPieces
								if err := rows.Scan(&seg.StreamID, &seg.Position, &seg.RemotePieces); err != nil {
									return Error.Wrap(err)
								}
								allSegments = append(allSegments, seg)
							}
							return nil
						})
					if err != nil {
						return Error.Wrap(err)
					}

					allNodes := map[storj.NodeID]struct{}{}
					for i := range allSegments {
						seg := &allSegments[i]
						for k := range seg.RemotePieces {
							p := &seg.RemotePieces[k]
							allNodes[p.StorageNode] = struct{}{}
						}
					}

					nodesList := []storj.NodeID{}
					for id := range allNodes {
						nodesList = append(nodesList, id)
					}
					aliasCache := NewNodeAliasCache(&txNodeAliases{tx})
					_, err = aliasCache.Aliases(ctx, nodesList)
					if err != nil {
						return Error.Wrap(err)
					}

					err = func() (err error) {
						stmt, err := tx.PrepareContext(ctx, `UPDATE segments SET remote_alias_pieces = $3 WHERE stream_id = $1 AND position = $2`)
						if err != nil {
							return Error.Wrap(err)
						}
						defer func() { err = errs.Combine(err, Error.Wrap(stmt.Close())) }()

						for i := range allSegments {
							seg := &allSegments[i]
							if len(seg.RemotePieces) == 0 {
								continue
							}

							aliases, err := aliasCache.ConvertPiecesToAliases(ctx, seg.RemotePieces)
							if err != nil {
								return Error.Wrap(err)
							}
							sort.Slice(aliases, func(i, k int) bool {
								return aliases[i].Number < aliases[k].Number
							})

							_, err = stmt.ExecContext(ctx, seg.StreamID, seg.Position, aliases)
							if err != nil {
								return Error.Wrap(err)
							}
						}

						return nil
					}()
					if err != nil {
						return err
					}

					return nil
				}),
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
