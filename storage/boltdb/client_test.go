// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/storj/storage"
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

type boltLongBenchmarkStore struct {
	*Client
	dirPath string
}

func (store *boltLongBenchmarkStore) BulkImport(ctx context.Context, iter storage.Iterator) (err error) {
	// turn off syncing during import
	oldval := store.db.NoSync
	store.db.NoSync = true
	defer func() { store.db.NoSync = oldval }()

	var item storage.ListItem
	for iter.Next(ctx, &item) {
		if err := store.Put(ctx, item.Key, item.Value); err != nil {
			return fmt.Errorf("Failed to insert data (%q, %q): %v", item.Key, item.Value, err)
		}
	}

	return store.db.Sync()
}

func (store *boltLongBenchmarkStore) BulkDeleteAll(ctx context.Context) error {
	// do nothing here; everything will be cleaned up later after the test completes. it's not
	// worth it to wait for BoltDB to remove every key, one by one, and we can't just
	// os.RemoveAll() the whole test directory at this point because those files are still open
	// and unremoveable on Windows.
	return nil
}

var _ testsuite.BulkImporter = &boltLongBenchmarkStore{}
var _ testsuite.BulkCleaner = &boltLongBenchmarkStore{}

func BenchmarkSuiteLong(b *testing.B) {
	tempdir, err := ioutil.TempDir("", "storj-bolt")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempdir); err != nil {
			b.Fatal(err)
		}
	}()

	dbname := filepath.Join(tempdir, "bolt.db")
	store, err := New(dbname, "bucket")
	if err != nil {
		b.Fatalf("failed to create db: %v", err)
	}
	defer func() {
		if err := errs.Combine(store.Close(), os.RemoveAll(tempdir)); err != nil {
			b.Fatalf("failed to close db: %v", err)
		}
	}()

	longStore := &boltLongBenchmarkStore{
		Client:  store,
		dirPath: tempdir,
	}
	testsuite.BenchmarkPathOperationsInLargeDb(b, longStore)
}

func BenchmarkClientWrite(b *testing.B) {
	// setup db
	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	dbfile := ctx.File("testbolt.db")
	dbs, err := NewShared(dbfile, "kbuckets", "nodes")
	if err != nil {
		b.Fatalf("failed to create db: %v\n", err)
	}
	defer func() {
		if err := dbs[0].Close(); err != nil {
			b.Fatalf("failed to close db: %v\n", err)
		}
		if err := dbs[1].Close(); err != nil {
			b.Fatalf("failed to close db: %v\n", err)
		}
	}()
	kdb := dbs[0]

	// benchmark test: execute 1000 Put operations where each call to `PutAndCommit` does the following:
	// 1) create a BoltDB transaction (tx), 2) execute the db operation, 3) commit the tx which writes it to disk.
	for n := 0; n < b.N; n++ {
		var group errgroup.Group
		for i := 0; i < 1000; i++ {
			key := storage.Key(fmt.Sprintf("testkey%d", i))
			value := storage.Value("testvalue")

			group.Go(func() error {
				return kdb.PutAndCommit(ctx, key, value)
			})
		}
		if err := group.Wait(); err != nil {
			b.Fatalf("PutAndCommit: %v", err)
		}
	}
}

func BenchmarkClientNoSyncWrite(b *testing.B) {
	// setup db
	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	dbfile := ctx.File("testbolt.db")
	dbs, err := NewShared(dbfile, "kbuckets", "nodes")
	if err != nil {
		b.Fatalf("failed to create db: %v\n", err)
	}
	defer func() {
		if err := dbs[0].Close(); err != nil {
			b.Fatalf("failed to close db: %v\n", err)
		}
		if err := dbs[1].Close(); err != nil {
			b.Fatalf("failed to close db: %v\n", err)
		}
	}()
	kdb := dbs[0]

	// benchmark test: execute 1000 Put operations with fsync turned off.
	// Each call to `PutAndCommit` does the following: 1) creates a BoltDB transaction (tx),
	// 2) executes the db operation, and 3) commits the tx which does NOT write it to disk.
	kdb.db.NoSync = true
	for n := 0; n < b.N; n++ {
		var group errgroup.Group
		for i := 0; i < 1000; i++ {
			key := storage.Key(fmt.Sprintf("testkey%d", i))
			value := storage.Value("testvalue")

			group.Go(func() error {
				return kdb.PutAndCommit(ctx, key, value)
			})
		}
		if err := group.Wait(); err != nil {
			b.Fatalf("PutAndCommit: %v", err)
		}
	}
	err = kdb.db.Sync()
	if err != nil {
		b.Fatalf("boltDB sync err: %v\n", err)
	}

}

func BenchmarkClientBatchWrite(b *testing.B) {
	// setup db
	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	dbfile := ctx.File("testbolt.db")
	dbs, err := NewShared(dbfile, "kbuckets", "nodes")
	if err != nil {
		b.Fatalf("failed to create db: %v\n", err)
	}
	defer func() {
		if err := dbs[0].Close(); err != nil {
			b.Fatalf("failed to close db: %v\n", err)
		}
		if err := dbs[1].Close(); err != nil {
			b.Fatalf("failed to close db: %v\n", err)
		}
	}()
	kdb := dbs[0]

	// benchmark test: batch 1000 Put operations.
	// Each call to `Put` does the following: 1) adds the db operation to a queue in boltDB,
	// 2) every 1000 operations or 10ms, whichever is first, BoltDB creates a single
	// transaction for all operations currently in the batch, executes the operations,
	// commits, and writes them to disk
	for n := 0; n < b.N; n++ {
		var group errgroup.Group
		for i := 0; i < 1000; i++ {
			key := storage.Key(fmt.Sprintf("testkey%d", i))
			value := storage.Value("testvalue")
			group.Go(func() error {
				return kdb.Put(ctx, key, value)
			})
		}
		if err := group.Wait(); err != nil {
			b.Fatalf("Put: %v", err)
		}
	}
}

func BenchmarkClientBatchNoSyncWrite(b *testing.B) {
	// setup db
	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	dbfile := ctx.File("testbolt.db")
	dbs, err := NewShared(dbfile, "kbuckets", "nodes")
	if err != nil {
		b.Fatalf("failed to create db: %v\n", err)
	}
	defer func() {
		if err := dbs[0].Close(); err != nil {
			b.Fatalf("failed to close db: %v\n", err)
		}
		if err := dbs[1].Close(); err != nil {
			b.Fatalf("failed to close db: %v\n", err)
		}
	}()
	kdb := dbs[0]

	// benchmark test: batch 1000 Put operations with fsync turned off.
	// Each call to `Put` does the following: 1) adds the db operation to a queue in boltDB,
	// 2) every 1000 operations or 2 ms, whichever is first, BoltDB creates a single
	// transaction for all operations currently in the batch, executes the operations,
	// commits, but does NOT write them to disk
	kdb.db.NoSync = true
	for n := 0; n < b.N; n++ {
		var group errgroup.Group
		for i := 0; i < 1000; i++ {
			key := storage.Key(fmt.Sprintf("testkey%d", i))
			value := storage.Value("testvalue")
			group.Go(func() error {
				return kdb.Put(ctx, key, value)
			})
		}

		if err := group.Wait(); err != nil {
			b.Fatalf("Put: %v", err)
		}

		err := kdb.db.Sync()
		if err != nil {
			b.Fatalf("boltDB sync err: %v\n", err)
		}
	}
}
