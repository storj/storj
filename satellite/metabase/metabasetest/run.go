// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/cfgstruct"
	"storj.io/common/dbutil/pgutil"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// RunWithConfig runs tests with specific metabase configuration.
func RunWithConfig(t *testing.T, config metabase.Config, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB)) {
	RunWithConfigAndMigration(t, config, fn, func(ctx context.Context, db *metabase.DB) error {
		return db.TestMigrateToLatest(ctx)
	})
}

// RunWithConfigAndMigration runs tests with specific metabase configuration and migration type.
func RunWithConfigAndMigration(t *testing.T, config metabase.Config, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB), migration func(ctx context.Context, db *metabase.DB) error) {
	for _, dbinfo := range satellitedbtest.Databases() {
		dbinfo := dbinfo
		t.Run(dbinfo.Name, func(t *testing.T) {
			t.Parallel()

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			// generate unique application name to filter out full table scan queries from other tests executions
			config := config
			config.ApplicationName += pgutil.CreateRandomTestingSchemaName(6)
			db, err := satellitedbtest.CreateMetabaseDB(ctx, zaptest.NewLogger(t), t.Name(), "M", 0, dbinfo.MetabaseDB, config)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			}()

			if err := migration(ctx, db); err != nil {
				t.Fatal(err)
			}

			fullScansBefore, err := fullTableScanQueries(ctx, db, config.ApplicationName)
			if err != nil {
				t.Fatal(err)
			}

			fn(ctx, t, db)

			fullScansAfter, err := fullTableScanQueries(ctx, db, config.ApplicationName)
			if err != nil {
				t.Fatal(err)
			}

			diff := cmp.Diff(fullScansBefore, fullScansAfter)
			if diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

// Run runs tests against all configured databases.
func Run(t *testing.T, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB)) {
	var config metainfo.Config
	cfgstruct.Bind(pflag.NewFlagSet("", pflag.PanicOnError), &config,
		cfgstruct.UseTestDefaults(),
	)

	RunWithConfig(t, metabase.Config{
		ApplicationName:  "satellite-metabase-test",
		MinPartSize:      config.MinPartSize,
		MaxNumberOfParts: config.MaxNumberOfParts,

		ServerSideCopy:         config.ServerSideCopy,
		ServerSideCopyDisabled: config.ServerSideCopyDisabled,
		UseListObjectsIterator: config.UseListObjectsIterator,

		TestingUniqueUnversioned: true,
	}, fn)
}

// Bench runs benchmark for all configured databases.
func Bench(b *testing.B, fn func(ctx *testcontext.Context, b *testing.B, db *metabase.DB)) {
	for _, dbinfo := range satellitedbtest.Databases() {
		dbinfo := dbinfo
		b.Run(dbinfo.Name, func(b *testing.B) {
			ctx := testcontext.New(b)
			defer ctx.Cleanup()
			db, err := satellitedbtest.CreateMetabaseDB(ctx, zaptest.NewLogger(b), b.Name(), "M", 0, dbinfo.MetabaseDB, metabase.Config{
				ApplicationName:  "satellite-bench",
				MinPartSize:      5 * memory.MiB,
				MaxNumberOfParts: 10000,
			})
			if err != nil {
				b.Fatal(err)
			}
			defer func() {
				if err := db.Close(); err != nil {
					b.Error(err)
				}
			}()

			if err := db.MigrateToLatest(ctx); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			fn(ctx, b, db)
		})
	}
}

func fullTableScanQueries(ctx context.Context, db *metabase.DB, applicationName string) (_ map[string]int, err error) {
	if db.Implementation().String() != "cockroach" {
		return nil, nil
	}

	rows, err := db.UnderlyingTagSQL().QueryContext(ctx,
		"SELECT key, count FROM crdb_internal.node_statement_statistics WHERE full_scan = TRUE AND application_name = $1 ORDER BY count DESC",
		applicationName,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	result := map[string]int{}
	for rows.Next() {
		var query string
		var count int
		err := rows.Scan(&query, &count)
		if err != nil {
			return nil, err
		}

		switch {
		case strings.Contains(query, "WITH ignore_full_scan_for_test AS (SELECT _)"):
			continue
		case !strings.Contains(strings.ToUpper(query), "WHERE"): // find smarter way to ignore known full table scan queries
			continue
		}

		result[query] += count
	}

	return result, rows.Err()
}
