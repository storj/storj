// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"storj.io/storj/storage"
)

func TestCommon(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "bolt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempdir)

	dbname := filepath.Join(tempdir, "bolt.db")
	store, err := NewClient(dbname, "bucket")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("failed to close db: %v", err)
		}
	}()

	storage.RunTests(t, storage.NewTestLogger(t, store))
}
