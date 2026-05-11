// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	_ "embed"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/migrate"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

var errTiDBNotSupported = errs.Class("tidb: not supported")

//go:embed adapter_tidb_scheme.sql
var tidbDDL string
var tidbDDLs = spannerutil.MustSplitSQLStatements(tidbDDL)

// TiDBAdapter implements Adapter for TiDB connections via the MySQL wire protocol.
type TiDBAdapter struct {
	log     *zap.Logger
	db      tagsql.DB
	connstr string

	config *Config

	aliasCache *NodeAliasCache
}

// NewTiDBAdapter creates a new TiDBAdapter from an already-opened tagsql.DB.
func NewTiDBAdapter(log *zap.Logger, db tagsql.DB, connstr string, config *Config, aliasCache *NodeAliasCache) *TiDBAdapter {
	a := &TiDBAdapter{
		log:     log.Named("tidb"),
		db:      db,
		connstr: connstr,
		config:  config,
	}
	if aliasCache != nil {
		a.aliasCache = aliasCache
	} else {
		a.aliasCache = NewNodeAliasCache(a, false)
	}
	return a
}

// Name returns the name of the adapter.
func (t *TiDBAdapter) Name() string { return "tidb" }

// Implementation returns the dbutil.Implementation code for the adapter.
func (t *TiDBAdapter) Implementation() dbutil.Implementation { return dbutil.TiDB }

// Config returns the metabase configuration.
func (t *TiDBAdapter) Config() *Config { return t.config }

// UnderlyingDB returns a handle to the underlying DB.
func (t *TiDBAdapter) UnderlyingDB() tagsql.DB { return t.db }

// Close closes the underlying database connection.
func (t *TiDBAdapter) Close() error { return t.db.Close() }

// Ping checks whether the connection is alive.
func (t *TiDBAdapter) Ping(ctx context.Context) error { return t.db.PingContext(ctx) }

// Now returns the current time according to TiDB.
func (t *TiDBAdapter) Now(ctx context.Context) (now time.Time, err error) {
	defer mon.Task()(&ctx)(&err)
	// NOW(6) returns microsecond precision, compared to NOW() second precision.
	row := t.db.QueryRowContext(ctx, `SELECT NOW(6)`)
	if err := row.Scan(&now); err != nil {
		return time.Time{}, Error.Wrap(err)
	}
	return now, nil
}

// TiDBMigration returns the migration steps needed for a TiDB metabase.
func (t *TiDBAdapter) TiDBMigration() *migrate.Migration {
	db := t.db
	return &migrate.Migration{
		Table: "tidb_metabase_versions",
		Steps: []*migrate.Step{
			{
				DB:          &db,
				Description: "initial setup",
				Version:     1,
				Action:      migrate.SQL(tidbDDLs),
			},
		},
	}
}

// MigrateToLatest migrates the database to the latest version.
func (t *TiDBAdapter) MigrateToLatest(ctx context.Context) error {
	return t.TiDBMigration().Run(ctx, t.log.Named("migrate"))
}

// CheckVersion checks that the database is at the expected migration version.
func (t *TiDBAdapter) CheckVersion(ctx context.Context) error {
	return t.TiDBMigration().ValidateVersions(ctx, t.log)
}

// TestMigrateToLatest applies the schema for tests.
func (t *TiDBAdapter) TestMigrateToLatest(ctx context.Context) error {
	if err := t.MigrateToLatest(ctx); err != nil {
		return err
	}

	if t.config != nil && t.config.TestingUniqueUnversioned {
		// TiDB does not support partial unique indexes (i.e. ... WHERE status
		// IN (...)) like PostgreSQL does. We emulate the same constraint with
		// a virtual generated column that is non-NULL only for unversioned
		// statuses (3=committed unversioned, 6=unversioned delete marker)
		// plus a regular UNIQUE index on it. Multiple NULL values are
		// permitted by a UNIQUE index per the SQL standard, so this only
		// forbids two unversioned rows at the same
		// (project_id, bucket_name, object_key).
		//
		// The column width 4080 = 16 (project_id) + 64 (bucket_name) +
		// 4000 (object_key) is the worst-case CONCAT length.
		stmts := []string{
			`ALTER TABLE objects
				ADD COLUMN unversioned_uniqueness VARBINARY(4080) AS (
					CASE WHEN status IN (3, 6)
						THEN CONCAT(project_id, bucket_name, object_key)
						ELSE NULL END
				) VIRTUAL`,
			`ALTER TABLE objects
				ADD UNIQUE INDEX objects_one_unversioned_per_location (unversioned_uniqueness)`,
		}
		for _, stmt := range stmts {
			if _, err := t.db.ExecContext(ctx, stmt); err != nil {
				// Tolerate "already exists" so re-running tests against the
				// same database is harmless.
				msg := err.Error()
				if !strings.Contains(msg, "Duplicate") &&
					!strings.Contains(msg, "already exists") {
					return Error.Wrap(err)
				}
			}
		}
	}

	return nil
}
