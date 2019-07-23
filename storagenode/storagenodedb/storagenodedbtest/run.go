// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedbtest

// This package should be referenced only in test files!

import (
	"testing"

	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb"
)

// Run method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Run(t *testing.T, test func(t *testing.T, db storagenode.DB)) {
	t.Run("Sqlite", func(t *testing.T) {
		t.Parallel()
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		log := zaptest.NewLogger(t)

		db, err := storagenodedb.NewTest(log, ctx.Dir("storage"))
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Check(db.Close)

		err = db.CreateTables()
		if err != nil {
			t.Fatal(err)
		}

		test(t, db)
	})
}
