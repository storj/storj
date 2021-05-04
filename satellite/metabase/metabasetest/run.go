// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"flag"
	"os"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

var databasesFlag = flag.String("databases", os.Getenv("STORJ_TEST_DATABASES"), "databases to use for testing")

// Database contains info about a test database connection.
type Database struct {
	Name    string
	Driver  string
	ConnStr string
}

// DatabaseEntries returns databases passed in with -databases or STORJ_TEST_DATABASES flag.
func DatabaseEntries() []Database {
	infos := []Database{
		{"pg", "pgx", "postgres://storj:storj-pass@localhost/metabase?sslmode=disable"},
		{"crdb", "pgx", "cockroach://root@localhost:26257/metabase?sslmode=disable"},
	}
	if *databasesFlag != "" {
		infos = nil
		for _, db := range strings.Split(*databasesFlag, ";") {
			toks := strings.Split(strings.TrimSpace(db), "|")
			infos = append(infos, Database{toks[0], toks[1], toks[2]})
		}
	}
	return infos
}

// Run runs tests against all configured databases.
func Run(t *testing.T, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB)) {
	for _, info := range DatabaseEntries() {
		info := info
		t.Run(info.Name, func(t *testing.T) {
			t.Parallel()

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			db, err := satellitedbtest.CreateMetabaseDB(ctx, zaptest.NewLogger(t), t.Name(), "M", 0, satellitedbtest.Database{
				Name:    info.Name,
				URL:     info.ConnStr,
				Message: "",
			})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			}()

			if err := db.MigrateToLatest(ctx); err != nil {
				t.Fatal(err)
			}

			fn(ctx, t, db.InternalImplementation().(*metabase.DB))
		})
	}
}

// Bench runs benchmark for all configured databases.
func Bench(b *testing.B, fn func(ctx *testcontext.Context, b *testing.B, db *metabase.DB)) {
	for _, info := range DatabaseEntries() {
		info := info
		b.Run(info.Name, func(b *testing.B) {
			ctx := testcontext.New(b)
			defer ctx.Cleanup()

			db, err := satellitedbtest.CreateMetabaseDB(ctx, zaptest.NewLogger(b), b.Name(), "M", 0, satellitedbtest.Database{
				Name:    info.Name,
				URL:     info.ConnStr,
				Message: "",
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
			fn(ctx, b, db.InternalImplementation().(*metabase.DB))
		})
	}
}
