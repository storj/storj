// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/zeebo/errs"
	"storj.io/storj/storage"
	"storj.io/storj/storage/testsuite"
)

func BenchmarkClientWrite(b *testing.B) {
	// setup db
	tempdir, err := ioutil.TempDir("", "storj-bolt")
	if err != nil {
		fmt.Println("err:", err)
	}
	defer func() { _ = os.RemoveAll(tempdir) }()
	dbname := filepath.Join(tempdir, "testbolt.db")
	dbs, err := NewShared(dbname, "kbuckets", "nodes")
	if err != nil {
		fmt.Printf("failed to create db: %v\n", err)
	}
	defer func() {
		if err := dbs[0].Close(); err != nil {
			fmt.Printf("failed to close db: %v\n", err)
		}
		if err := dbs[1].Close(); err != nil {
			fmt.Printf("failed to close db: %v\n", err)
		}
	}()
	kdb := dbs[0]
	kdb.db.NoSync = false

	// benchmark test: execute 1000 Put operations
	for n := 0; n < b.N; n++ {
		for i := 0; i < 1000; i++ {
			key := storage.Key(fmt.Sprintf("testkey%d", i))
			value := storage.Value("testvalue")
			err := kdb.Put(key, value)
			if err != nil {
				fmt.Println("Put err:", err)
			}
		}
	}
	b.Logf("\n b.N: %d, TxStats Write: %v, WriteTime: %v\n", b.N, kdb.db.Stats().TxStats.Write, kdb.db.Stats().TxStats.WriteTime)
}

func BenchmarkClientNoSyncWrite(b *testing.B) {
	// setup db
	tempdir, err := ioutil.TempDir("", "storj-bolt")
	if err != nil {
		fmt.Println("err:", err)
	}
	defer func() { _ = os.RemoveAll(tempdir) }()
	dbname := filepath.Join(tempdir, "testbolt.db")
	dbs, err := NewShared(dbname, "kbuckets", "nodes")
	if err != nil {
		fmt.Printf("failed to create db: %v\n", err)
	}
	defer func() {
		if err := dbs[0].Close(); err != nil {
			fmt.Printf("failed to close db: %v\n", err)
		}
		if err := dbs[1].Close(); err != nil {
			fmt.Printf("failed to close db: %v\n", err)
		}
	}()
	kdb := dbs[0]

	// run benchmark test: execute 1000 Put operations with fsync turned off
	kdb.db.NoSync = true
	for n := 0; n < b.N; n++ {
		for i := 0; i < 1000; i++ {
			key := storage.Key(fmt.Sprintf("testkey%d", i))
			value := storage.Value("testvalue")
			err := kdb.Put(key, value)
			if err != nil {
				fmt.Println("Put Nosync err:", err)
			}
		}
	}
	kdb.db.Sync()
	b.Logf("\n b.N: %d, TxStats Write: %v, WriteTime: %v\n", b.N, kdb.db.Stats().TxStats.Write, kdb.db.Stats().TxStats.WriteTime)
}

func BenchmarkClientBatchWrite(b *testing.B) {
	// setup db
	tempdir, err := ioutil.TempDir("", "storj-bolt")
	if err != nil {
		fmt.Println("err:", err)
	}
	defer func() { _ = os.RemoveAll(tempdir) }()
	dbname := filepath.Join(tempdir, "batchbolt.db")
	dbs, err := NewShared(dbname, "kbuckets", "nodes")
	if err != nil {
		fmt.Printf("failed to create db: %v\n", err)
	}
	kdb := dbs[0]
	kdb.db.NoSync = false

	// run benchmark tests: execute 1000 Put operations with batch
	for n := 0; n < b.N; n++ {
		for i := 0; i < 1000; i++ {
			key := storage.Key(fmt.Sprintf("testkey%d", i))
			value := storage.Value("testvalue")

			err := kdb.BatchPut(key, value)
			if err != nil {
				fmt.Println("batch err: ", err)
			}
		}
	}
	b.Logf("\n b.N: %d, TxStats Write: %v, WriteTime: %v\n", b.N, kdb.db.Stats().TxStats.Write, kdb.db.Stats().TxStats.WriteTime)
}

func BenchmarkClientBatchNoSyncWrite(b *testing.B) {
	// setup db
	tempdir, err := ioutil.TempDir("", "storj-bolt")
	if err != nil {
		fmt.Println("err:", err)
	}
	defer func() { _ = os.RemoveAll(tempdir) }()
	dbname := filepath.Join(tempdir, "batchbolt.db")
	dbs, err := NewShared(dbname, "kbuckets", "nodes")
	if err != nil {
		fmt.Printf("failed to create db: %v\n", err)
	}
	kdb := dbs[0]
	kdb.db.NoSync = true

	// run benchmark tests: execute 1000 Put operations with batch and no fsync
	for n := 0; n < b.N; n++ {
		for i := 0; i < 1000; i++ {
			key := storage.Key(fmt.Sprintf("testkey%d", i))
			value := storage.Value("testvalue")

			err := kdb.BatchPut(key, value)
			if err != nil {
				fmt.Println("batch err: ", err)
			}
		}
	}
	kdb.db.Sync()
	b.Logf("\n b.N: %d, TxStats Write: %v, WriteTime: %v\n", b.N, kdb.db.Stats().TxStats.Write, kdb.db.Stats().TxStats.WriteTime)
}

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

func (store *boltLongBenchmarkStore) BulkImport(iter storage.Iterator) (err error) {
	// turn off syncing during import
	oldval := store.db.NoSync
	store.db.NoSync = true
	defer func() { store.db.NoSync = oldval }()

	var item storage.ListItem
	for iter.Next(&item) {
		if err := store.Put(item.Key, item.Value); err != nil {
			return fmt.Errorf("Failed to insert data (%q, %q): %v", item.Key, item.Value, err)
		}
	}

	return store.db.Sync()
}

func (store *boltLongBenchmarkStore) BulkDelete() error {
	// do nothing here; everything will be cleaned up later after the test completes. it's not
	// worth it to wait for BoltDB to remove every key, one by one, and we can't just
	// os.RemoveAll() the whole test directory at this point because those files are still open
	// and unremoveable on Windows.
	return nil
}

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
