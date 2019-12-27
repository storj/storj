// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedbtest

// This package should be referenced only in test files!

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/mattn/go-sqlite3"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/utccheck"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb"
)

func init() {
	sql.Register("sqlite3+utccheck", utccheck.WrapDriver(&sqlite3.SQLiteDriver{}))
}

// Run method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Run(t *testing.T, test func(t *testing.T, db storagenode.DB)) {
	t.Run("Sqlite", func(t *testing.T) {
		t.Parallel()
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		log := zaptest.NewLogger(t)

		storageDir := ctx.Dir("storage")
		cfg := storagenodedb.Config{
			Storage: storageDir,
			Info:    filepath.Join(storageDir, "piecestore.db"),
			Info2:   filepath.Join(storageDir, "info.db"),
			Driver:  "sqlite3+utccheck",
			Pieces:  storageDir,
		}

		db, err := storagenodedb.New(log, cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Check(db.Close)

		err = db.CreateTables(ctx)
		if err != nil {
			t.Fatal(err)
		}

		test(t, db)
	})
}
