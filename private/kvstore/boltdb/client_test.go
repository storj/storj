// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"path/filepath"
	"testing"

	"storj.io/storj/private/kvstore/testsuite"
)

func TestSuite(t *testing.T) {
	dbname := filepath.Join(t.TempDir(), "bolt.db")
	store, err := New(dbname, "bucket")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("failed to close db: %v", err)
		}
	}()

	testsuite.RunTests(t, store)
}

func BenchmarkSuite(b *testing.B) {
	dbname := filepath.Join(b.TempDir(), "bolt.db")
	store, err := New(dbname, "bucket")
	if err != nil {
		b.Fatalf("failed to create db: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			b.Fatalf("failed to close db: %v", err)
		}
	}()

	testsuite.RunBenchmarks(b, store)
}
