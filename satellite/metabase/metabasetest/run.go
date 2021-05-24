// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"testing"

	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// Run runs tests against all configured databases.
func Run(t *testing.T, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB)) {
	for _, dbinfo := range satellitedbtest.Databases() {
		dbinfo := dbinfo
		t.Run(dbinfo.Name, func(t *testing.T) {
			t.Parallel()

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			db, err := satellitedbtest.CreateMetabaseDB(ctx, zaptest.NewLogger(t), t.Name(), "M", 0, dbinfo.MetabaseDB)
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
	for _, dbinfo := range satellitedbtest.Databases() {
		dbinfo := dbinfo
		b.Run(dbinfo.Name, func(b *testing.B) {
			ctx := testcontext.New(b)
			defer ctx.Cleanup()

			db, err := satellitedbtest.CreateMetabaseDB(ctx, zaptest.NewLogger(b), b.Name(), "M", 0, dbinfo.MetabaseDB)
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
