// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build go1.25

package hashstore

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestDB_BackgroundCompactionLoop(t *testing.T) {
	forAllTables(t, testDB_BackgroundCompactionLoop)
}

func testDB_BackgroundCompactionLoop(t *testing.T, cfg Config) {
	synctest.Test(t, func(t *testing.T) {
		db := newTestDB(t, cfg)
		defer db.Close()

		for func() bool {
			db, s0, s1 := db.Stats()
			return db.Compactions < 10 || s0.Compactions < 5 || s1.Compactions < 5
		}() {
			time.Sleep(time.Hour)
		}
	})
}
