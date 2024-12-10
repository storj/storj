// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package metabase implements storing objects and segements.
package metabase

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/spanner"
	_ "github.com/googleapis/go-sql-spanner" // registers spanner as a tagsql driver.
	_ "github.com/jackc/pgx/v5"              // registers pgx as a tagsql driver.
	_ "github.com/jackc/pgx/v5/stdlib"       // registers pgx as a tagsql driver.
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/uuid"
	"storj.io/storj/private/logging"
	"storj.io/storj/private/migrate"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

var (
	mon = monkit.Package()
)

// Config is a configuration struct for part validation.
type Config struct {
	ApplicationName  string
	MinPartSize      memory.Size
	MaxNumberOfParts int

	// TODO remove this flag when server-side copy implementation will be finished
	ServerSideCopy         bool
	ServerSideCopyDisabled bool
	UseListObjectsIterator bool

	NodeAliasCacheFullRefresh bool

	TestingUniqueUnversioned   bool
	TestingPrecommitDeleteMode TestingPrecommitDeleteMode
	TestingSpannerProjects     map[uuid.UUID]struct{}
}

// DB implements a database for storing objects and segments.
type DB struct {
	log *zap.Logger
	db  tagsql.DB

	aliasCache *NodeAliasCache

	testCleanup func() error

	config Config

	adapters []Adapter

	projectsAdapters map[uuid.UUID]Adapter
}

// Open opens a connection to metabase.
func Open(ctx context.Context, log *zap.Logger, connstr string, config Config) (*DB, error) {
	db := &DB{
		log:         log,
		testCleanup: func() error { return nil },
		config:      config,
	}
	db.aliasCache = NewNodeAliasCache(db, config.NodeAliasCacheFullRefresh)

	connStrs := strings.Split(connstr, ";")
	if len(connStrs) == 0 {
		return nil, Error.New("no connection strings provided")
	}

	db.adapters = make([]Adapter, len(connStrs))
	db.projectsAdapters = make(map[uuid.UUID]Adapter)

	for i, connstr := range connStrs {
		_, source, impl, err := dbutil.SplitConnStr(connstr)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		connstr, err = pgutil.EnsureApplicationName(connstr, config.ApplicationName)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		switch impl {
		case dbutil.Postgres:
			rawdb, err := tagsql.Open(ctx, "pgx", connstr)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			dbutil.Configure(ctx, rawdb, "metabase", mon)

			db.db = postgresRebind{rawdb}
			db.adapters[i] = &PostgresAdapter{
				log:                      log,
				db:                       db.db,
				impl:                     impl,
				connstr:                  connstr,
				testingUniqueUnversioned: config.TestingUniqueUnversioned,
			}
		case dbutil.Cockroach:
			rawdb, err := tagsql.Open(ctx, "cockroach", connstr)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			dbutil.Configure(ctx, rawdb, "metabase", mon)

			db.db = postgresRebind{rawdb}
			db.adapters[i] = &CockroachAdapter{
				PostgresAdapter{
					log:                      log,
					db:                       db.db,
					impl:                     impl,
					connstr:                  connstr,
					testingUniqueUnversioned: config.TestingUniqueUnversioned,
				},
			}
		case dbutil.Spanner:
			adapter, err := NewSpannerAdapter(ctx, SpannerConfig{
				Database:        source,
				ApplicationName: config.ApplicationName,
			}, log)
			if err != nil {
				return nil, err
			}
			db.adapters[i] = adapter
			for projectID := range config.TestingSpannerProjects {
				db.projectsAdapters[projectID] = adapter
			}
		default:
			return nil, Error.New("unsupported implementation: %s", connstr)
		}

		if log.Level() == zap.DebugLevel {
			log.Debug("Connected", zap.String("db source", logging.Redacted(connstr)), zap.Int("db adapter ordinal", i))
		}
	}

	return db, nil
}

// Implementation returns the implementation for the first db adapter.
// TODO: remove this.
func (db *DB) Implementation() dbutil.Implementation {
	return db.adapters[0].Implementation()
}

// ChooseAdapter selects the right adapter based on configuration.
func (db *DB) ChooseAdapter(projectID uuid.UUID) Adapter {
	if adapter, ok := db.projectsAdapters[projectID]; ok {
		return adapter
	}
	return db.adapters[0]
}

// UnderlyingTagSQL returns *tagsql.DB.
// TODO: remove.
func (db *DB) UnderlyingTagSQL() tagsql.DB { return db.db }

// Ping checks whether connection has been established to all adapters.
func (db *DB) Ping(ctx context.Context) error {
	for _, adapter := range db.adapters {
		err := adapter.Ping(ctx)
		if err != nil {
			return Error.Wrap(err)
		}
	}
	return nil
}

// Ping checks whether connection has been established.
func (p *PostgresAdapter) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

// Ping checks whether connection has been established.
func (s *SpannerAdapter) Ping(ctx context.Context) error {
	ok, err := spannerutil.CollectRow(s.client.Single().Query(ctx, spanner.Statement{SQL: `SELECT true`}),
		func(row *spanner.Row, item *bool) error {
			return row.Columns(item)
		})
	if err != nil {
		return Error.Wrap(err)
	}
	if !ok {
		return Error.New("up is down, left is right, true is false, and forwards is backwards")
	}
	return nil
}

// TestingSetCleanup is used to set the callback for cleaning up test database.
func (db *DB) TestingSetCleanup(cleanup func() error) {
	db.testCleanup = cleanup
}

// Close closes the connection to database.
func (db *DB) Close() error {
	var err error
	if db.db != nil {
		err = Error.Wrap(db.db.Close())
	}
	for _, adapter := range db.adapters {
		if c, isCloser := adapter.(io.Closer); isCloser {
			err = errs.Combine(err, Error.Wrap(c.Close()))
		}
	}
	return errs.Combine(err, db.testCleanup())
}

// DestroyTables deletes all tables.
//
// TODO: remove this, only for bootstrapping.
func (db *DB) DestroyTables(ctx context.Context) error {
	_, err := db.db.ExecContext(ctx, `
		DROP TABLE IF EXISTS objects;
		DROP TABLE IF EXISTS segments;
		DROP TABLE IF EXISTS node_aliases;
		DROP TABLE IF EXISTS metabase_versions;
		DROP SEQUENCE IF EXISTS node_alias_seq;
	`)
	db.aliasCache.reset()
	return Error.Wrap(err)
}

// TestMigrateToLatest replaces the migration steps with only one step to create metabase db.
// It is applied to all db adapters.
func (db *DB) TestMigrateToLatest(ctx context.Context) error {
	for _, a := range db.adapters {
		err := a.TestMigrateToLatest(ctx)
		if err != nil {
			return Error.Wrap(err)
		}
	}
	return nil
}

// MigrateToLatest migrates database to the latest version.
func (db *DB) MigrateToLatest(ctx context.Context) error {
	for _, a := range db.adapters {
		err := a.MigrateToLatest(ctx)
		if err != nil {
			return Error.Wrap(err)
		}
	}
	return nil
}

// MigrateToLatest migrates database to the latest version.
func (p *PostgresAdapter) MigrateToLatest(ctx context.Context) error {
	// First handle the idiosyncrasies of postgres and cockroach migrations. Postgres
	// will need to create any schemas specified in the search path, and cockroach
	// will need to create the database it was told to connect to. These things should
	// not really be here, and instead should be assumed to exist.
	// This is tracked in jira ticket SM-200
	schema, err := pgutil.ParseSchemaFromConnstr(p.connstr)
	if err != nil {
		return errs.New("error parsing schema: %+v", err)
	}

	if schema != "" {
		err = pgutil.CreateSchema(ctx, p.db, schema)
		if err != nil {
			return errs.New("error creating schema: %+v", err)
		}
	}
	migration := p.PostgresMigration()
	return migration.Run(ctx, p.log.Named("migrate"))
}

// MigrateToLatest migrates database to the latest version.
func (c *CockroachAdapter) MigrateToLatest(ctx context.Context) error {
	var dbName string
	if err := c.db.QueryRowContext(ctx, `SELECT current_database();`).Scan(&dbName); err != nil {
		return errs.New("error querying current database: %+v", err)
	}

	_, err := c.db.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE IF NOT EXISTS %s;`,
		pgutil.QuoteIdentifier(dbName)))
	if err != nil {
		return errs.Wrap(err)
	}
	migration := c.PostgresMigration()
	return migration.Run(ctx, c.log.Named("migrate"))
}

// MigrateToLatest migrates database to the latest version.
func (s *SpannerAdapter) MigrateToLatest(ctx context.Context) error {
	migration := s.SpannerMigration()
	return migration.Run(ctx, s.log.Named("migrate"))
}

// CheckVersion checks the database is the correct version.
func (db *DB) CheckVersion(ctx context.Context) error {
	for _, a := range db.adapters {
		err := a.CheckVersion(ctx)
		if err != nil {
			return Error.Wrap(err)
		}
	}
	return nil
}

// CheckVersion checks the database is the correct version.
func (p *PostgresAdapter) CheckVersion(ctx context.Context) error {
	migration := p.PostgresMigration()
	return migration.ValidateVersions(ctx, p.log)
}

// CheckVersion checks the database is the correct version.
func (s *SpannerAdapter) CheckVersion(ctx context.Context) error {
	migration := s.SpannerMigration()
	return migration.ValidateVersions(ctx, s.log)
}

// PostgresMigration returns steps needed for migrating postgres database.
func (p *PostgresAdapter) PostgresMigration() *migrate.Migration {
	var db tagsql.DB = postgresRebind{p.db}

	// TODO: merge this with satellite migration code or a way to keep them in sync.
	return &migrate.Migration{
		Table: "metabase_versions",
		Steps: []*migrate.Step{
			{
				DB:          &db,
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

						status         INT2 NOT NULL default ` + statusPending + `,
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
				DB:          &db,
				Description: "change total_plain_size and total_encrypted_size to INT8",
				Version:     2,
				Action: migrate.SQL{
					`ALTER TABLE objects ALTER COLUMN total_plain_size TYPE INT8;`,
					`ALTER TABLE objects ALTER COLUMN total_encrypted_size TYPE INT8;`,
				},
			},
			{
				DB:          &db,
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
				DB:          &db,
				Description: "add remote_alias_pieces column",
				Version:     4,
				Action: migrate.SQL{
					`ALTER TABLE segments ADD COLUMN remote_alias_pieces BYTEA`,
				},
			},
			{
				DB:          &db,
				Description: "drop remote_pieces from segments table",
				Version:     6,
				Action: migrate.SQL{
					`ALTER TABLE segments DROP COLUMN remote_pieces`,
				},
			},
			{
				DB:          &db,
				Description: "add created_at and repaired_at columns to segments table",
				Version:     7,
				Action: migrate.SQL{
					`ALTER TABLE segments ADD COLUMN created_at TIMESTAMPTZ`,
					`ALTER TABLE segments ADD COLUMN repaired_at TIMESTAMPTZ`,
				},
			},
			{
				DB:          &db,
				Description: "change default of created_at column in segments table to now()",
				Version:     8,
				Action: migrate.SQL{
					`ALTER TABLE segments ALTER COLUMN created_at SET DEFAULT now()`,
				},
			},
			{
				DB:          &db,
				Description: "add encrypted_etag column to segments table",
				Version:     9,
				Action: migrate.SQL{
					`ALTER TABLE segments ADD COLUMN encrypted_etag BYTEA default NULL`,
				},
			},
			{
				DB:          &db,
				Description: "add index on pending objects",
				Version:     10,
				Action:      migrate.SQL{},
			},
			{
				DB:          &db,
				Description: "drop pending_index on objects",
				Version:     11,
				Action: migrate.SQL{
					`DROP INDEX IF EXISTS pending_index`,
				},
			},
			{
				DB:          &db,
				Description: "add expires_at column to segments",
				Version:     12,
				Action: migrate.SQL{
					`ALTER TABLE segments ADD COLUMN expires_at TIMESTAMPTZ`,
				},
			},
			{
				DB:          &db,
				Description: "add NOT NULL constraint to created_at column in segments table",
				Version:     13,
				Action: migrate.SQL{
					`ALTER TABLE segments ALTER COLUMN created_at SET NOT NULL`,
				},
			},
			{
				DB:          &db,
				Description: "ADD placement to the segments table",
				Version:     14,
				Action: migrate.SQL{
					`ALTER TABLE segments ADD COLUMN placement integer`,
				},
			},
			{
				DB:          &db,
				Description: "add table for segment copies",
				Version:     15,
				Action: migrate.SQL{
					`CREATE TABLE segment_copies (
						stream_id BYTEA NOT NULL PRIMARY KEY,
						ancestor_stream_id BYTEA NOT NULL,

						CONSTRAINT not_self_ancestor CHECK (stream_id != ancestor_stream_id)
					)`,
					`CREATE INDEX ON segment_copies (ancestor_stream_id)`,
				},
			},
			{
				DB:          &db,
				Description: "add database comments",
				Version:     16,
				Action: migrate.SQL{`
					-- objects table
					COMMENT ON TABLE  objects             is 'Objects table contains information about path and streams.';
					COMMENT ON COLUMN objects.project_id  is 'project_id is a uuid referring to project.id.';
					COMMENT ON COLUMN objects.bucket_name is 'bucket_name is a alpha-numeric string referring to bucket_metainfo.name.';
					COMMENT ON COLUMN objects.object_key  is 'object_key is an encrypted path of the object.';
					COMMENT ON COLUMN objects.version     is 'version is a monotonically increasing number per object. currently unused.';
					COMMENT ON COLUMN objects.stream_id   is 'stream_id is a random identifier for the content uploaded to the object.';

					COMMENT ON COLUMN objects.created_at  is 'created_at is the creation date of this object.';
					COMMENT ON COLUMN objects.expires_at  is 'expires_at is the date when this object will be marked for deletion.';

					COMMENT ON COLUMN objects.status        is 'status refers to metabase.ObjectStatus, where pending=1 and committed=3.';
					COMMENT ON COLUMN objects.segment_count is 'segment_count indicates, how many segments are in the segments table for this object. This is zero until the object is committed.';

					COMMENT ON COLUMN objects.encrypted_metadata_nonce is 'encrypted_metadata_nonce is random identifier used as part of encryption for encrypted_metadata.';
					COMMENT ON COLUMN objects.encrypted_metadata       is 'encrypted_metadata is encrypted key-value pairs of user-specified data.';
					COMMENT ON COLUMN objects.encrypted_metadata_encrypted_key is 'encrypted_metadata_encrypted_key is the encrypted key for encrypted_metadata.';

					COMMENT ON COLUMN objects.total_plain_size     is 'total_plain_size is the user-specified total size of the object. This can be zero for old migrated objects.';
					COMMENT ON COLUMN objects.total_encrypted_size is 'total_encrypted_size is the sum of the encrypted data sizes of segments.';
					COMMENT ON COLUMN objects.fixed_segment_size   is 'fixed_segment_size is specified for objects that have a uniform segment sizes (except the last segment). This can be zero for old migrated objects.';

					COMMENT ON COLUMN objects.encryption is 'encryption contains object encryption parameters encoded into a uint32. See metabase.encryptionParameters type for the implementation.';

					COMMENT ON COLUMN objects.zombie_deletion_deadline is 'zombie_deletion_deadline defines when a pending object can be deleted due to a failed upload.';

					-- segments table
					COMMENT ON TABLE  segments            is 'segments table contains where segment data is located and other metadata about them.';
					COMMENT ON COLUMN segments.stream_id  is 'stream_id is a uuid referring to segments that belong to the same object.';
					COMMENT ON COLUMN segments.position   is 'position is a segment sequence number, determining the order they should be read in. It is represented as uint64, where the upper 32bits indicate the part-number and the lower 32bits indicate the index inside the part.';

					COMMENT ON COLUMN segments.root_piece_id       is 'root_piece_id is used for deriving per storagenode piece numbers.';
					COMMENT ON COLUMN segments.encrypted_key_nonce is 'encrypted_key_nonce is random data used for encrypting the encrypted_key.';
					COMMENT ON COLUMN segments.encrypted_key       is 'encrypted_key is the encrypted key that was used for encrypting the data in this segment.';
					COMMENT ON COLUMN segments.remote_alias_pieces is 'remote_alias_pieces is a compressed list of storagenodes that contain the pieces. See metabase.AliasPieces to see how they are compressed.';

					COMMENT ON COLUMN segments.encrypted_size is 'encrypted_size is the data size after compression, but before Reed-Solomon encoding.';
					COMMENT ON COLUMN segments.plain_offset   is 'plain_offset is the offset of this segment in the unencrypted data stream. Old migrated objects do not have this information, and is zero.';
					COMMENT ON COLUMN segments.plain_size     is 'plain_size is the user-specified unencrypted size of this segment. Old migrated objects do not have this information, and it is zero.';

					COMMENT ON COLUMN segments.redundancy  is 'redundancy is the compressed Reed-Solomon redundancy parameters for this segment. See metabase.redundancyScheme for the compression.';

					COMMENT ON COLUMN segments.inline_data is 'inline_data contains encrypted data for small objects.';

					COMMENT ON COLUMN segments.created_at  is 'created_at is the date when the segment was committed to the table.';
					COMMENT ON COLUMN segments.repaired_at is 'repaired_at is the last date when the segment was repaired.';
					COMMENT ON COLUMN segments.expires_at  is 'expires_at is the date when the segment is marked for deletion.';

					COMMENT ON COLUMN segments.placement is 'placement is the country or region restriction for the segment data. See storj.PlacementConstraint for the values.';
					COMMENT ON COLUMN segments.encrypted_etag is 'encrypted_etag is etag that has been encrypted.';

					-- node aliases table
					COMMENT ON TABLE  node_aliases            is 'node_aliases table contains unique identifiers (aliases) for storagenodes that take less space than a NodeID.';
					COMMENT ON COLUMN node_aliases.node_id    is 'node_id refers to the storj.NodeID';
					COMMENT ON COLUMN node_aliases.node_alias is 'node_alias is a unique integer value assigned for the node_id. It is used for compressing segments.remote_alias_pieces.';

					-- segment copies table
					COMMENT ON TABLE  segment_copies                    is 'segment_copies contains a reference for sharing stream_id-s.';
					COMMENT ON COLUMN segment_copies.stream_id          is 'stream_id refers to the objects.stream_id.';
					COMMENT ON COLUMN segment_copies.ancestor_stream_id is 'ancestor_stream_id refers to the actual segments where data is stored.';
				`},
			},
			{
				DB:          &db,
				Description: "add pending_objects table",
				Version:     17,
				Action: migrate.SQL{`
					CREATE TABLE pending_objects (
						project_id   BYTEA NOT NULL,
						bucket_name  BYTEA NOT NULL,
						object_key   BYTEA NOT NULL,
						stream_id    BYTEA NOT NULL,

						created_at TIMESTAMPTZ NOT NULL default now(),
						expires_at TIMESTAMPTZ,

						encrypted_metadata_nonce         BYTEA default NULL,
						encrypted_metadata               BYTEA default NULL,
						encrypted_metadata_encrypted_key BYTEA default NULL,

						encryption INT8 NOT NULL default 0,

						zombie_deletion_deadline TIMESTAMPTZ default now() + '1 day',

						PRIMARY KEY (project_id, bucket_name, object_key, stream_id)
					)`,
					`
					COMMENT ON TABLE  pending_objects     is 'Pending objects table contains information about path and streams of in progress uploads';
					COMMENT ON COLUMN objects.project_id  is 'project_id is a uuid referring to project.id.';
					COMMENT ON COLUMN objects.bucket_name is 'bucket_name is a alpha-numeric string referring to bucket_metainfo.name.';
					COMMENT ON COLUMN objects.object_key  is 'object_key is an encrypted path of the object.';
					COMMENT ON COLUMN objects.stream_id   is 'stream_id is a random identifier for the content uploaded to the object.';

					COMMENT ON COLUMN objects.created_at  is 'created_at is the creation date of this object.';
					COMMENT ON COLUMN objects.expires_at  is 'expires_at is the date when this object will be marked for deletion.';

					COMMENT ON COLUMN objects.encrypted_metadata_nonce is 'encrypted_metadata_nonce is random identifier used as part of encryption for encrypted_metadata.';
					COMMENT ON COLUMN objects.encrypted_metadata       is 'encrypted_metadata is encrypted key-value pairs of user-specified data.';
					COMMENT ON COLUMN objects.encrypted_metadata_encrypted_key is 'encrypted_metadata_encrypted_key is the encrypted key for encrypted_metadata.';

					COMMENT ON COLUMN objects.encryption is 'encryption contains object encryption parameters encoded into a uint32. See metabase.encryptionParameters type for the implementation.';

					COMMENT ON COLUMN objects.zombie_deletion_deadline is 'zombie_deletion_deadline defines when a pending object can be deleted due to a failed upload.';
				`},
			},
			{
				DB:          &db,
				Description: "change objects.version from INT4 to INT8",
				Version:     18,
				Action: migrate.SQL{`
					-- change type from INT4 to INT8; this is practically instant on cockroachdb because
					-- it uses INT8 storage for INT4 values already.
					ALTER TABLE objects ALTER COLUMN version TYPE INT8;
				`},
			},
			{
				DB:          &db,
				Description: "add retention_mode and retain_until columns to objects table",
				Version:     19,
				Action: migrate.SQL{
					`ALTER TABLE objects ADD COLUMN retention_mode INT2`,
					`ALTER TABLE objects ADD COLUMN retain_until TIMESTAMPTZ`,
					`
					COMMENT ON COLUMN objects.retention_mode is 'retention_mode specifies an object version''s retention mode: NULL/0=none, and 1=compliance.';
					COMMENT ON COLUMN objects.retain_until   is 'retain_until specifies when an object version''s retention period ends.';
				`},
			},
			{
				DB:          &db,
				Description: "drop tables, pending_objects and segment_copies",
				Version:     20,
				Action: migrate.SQL{
					`DROP TABLE IF EXISTS pending_objects`,
					`DROP TABLE IF EXISTS segment_copies`,
				},
			},
			{
				DB:          &db,
				Description: "add clear_metadata field to objects table",
				Version:     21,
				Action: migrate.SQL{
					`ALTER TABLE objects ADD COLUMN clear_metadata JSONB`,
					`CREATE INDEX ON objects USING GIN (project_id, bucket_name, clear_metadata)`,
					`
					COMMENT ON COLUMN objects.clear_metadata is 'clear_metadata contains unencrypted metadata that indexed for efficient metadata search.';
				`},
			},
		},
	}
}

// SpannerMigration returns steps needed for migrating spanner database.
func (s *SpannerAdapter) SpannerMigration() *migrate.Migration {
	db := s.sqlClient

	var firstStepDDL []string
	for _, statement := range strings.Split(spannerDDL, ";") {
		if strings.TrimSpace(statement) != "" {
			firstStepDDL = append(firstStepDDL, statement)
		}
	}

	// TODO: merge this with satellite migration code or a way to keep them in sync.
	return &migrate.Migration{
		Table: "spanner_metabase_versions",
		Steps: []*migrate.Step{
			{
				DB:          &db,
				Description: "initial setup",
				Version:     1,
				Action:      migrate.SQL(firstStepDDL),
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

// Now returns the current time according to the first database adapter.
func (db *DB) Now(ctx context.Context) (time.Time, error) {
	return db.adapters[0].Now(ctx)
}

// Now returns the current time according to the database.
func (p *PostgresAdapter) Now(ctx context.Context) (time.Time, error) {
	var t time.Time
	err := p.db.QueryRowContext(ctx, `SELECT now()`).Scan(&t)
	return t, Error.Wrap(err)
}

// Now returns the current time according to the database.
func (s *SpannerAdapter) Now(ctx context.Context) (time.Time, error) {
	return spannerutil.CollectRow(
		s.client.Single().Query(ctx, spanner.Statement{SQL: `SELECT CURRENT_TIMESTAMP`}),
		func(row *spanner.Row, now *time.Time) error {
			return row.Columns(now)
		},
	)
}

// LimitedAsOfSystemTime returns a SQL query clause for AS OF SYSTEM TIME.
func LimitedAsOfSystemTime(impl dbutil.Implementation, now, baseline time.Time, maxInterval time.Duration) string {
	if baseline.IsZero() || now.IsZero() {
		return impl.AsOfSystemInterval(maxInterval)
	}

	interval := now.Sub(baseline)
	if interval < 0 {
		return ""
	}
	// maxInterval is negative
	if maxInterval < 0 && interval > -maxInterval {
		return impl.AsOfSystemInterval(maxInterval)
	}
	return impl.AsOfSystemTime(baseline)
}
