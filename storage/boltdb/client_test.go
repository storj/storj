// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"storj.io/storj/storage/testsuite"
)

func TestSuite(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "storj-bolt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempdir) }()

	dbname := filepath.Join(tempdir, "bolt.db")
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
	tempdir, err := ioutil.TempDir("", "storj-bolt")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempdir) }()

	dbname := filepath.Join(tempdir, "bolt.db")
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

func TestSuiteShared(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "storj-bolt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempdir) }()

	dbname := filepath.Join(tempdir, "bolt.db")
	stores, err := NewShared(dbname, "alpha", "beta")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer func() {
		for _, store := range stores {
			if err := store.Close(); err != nil {
				t.Fatalf("failed to close db: %v", err)
			}
		}
	}()

	for _, store := range stores {
		testsuite.RunTests(t, store)
	}
}
