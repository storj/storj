// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
)

func newTestStore(t testing.TB) (dir string, store *filestore.Store, cleanup func()) {
	dir, err := ioutil.TempDir("", "filestore")
	if err != nil {
		t.Fatal(err)
	}

	store, err = filestore.NewAt(dir)
	if err != nil {
		t.Fatal(err)
	}

	return dir, store, func() {
		err := os.RemoveAll(dir)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestStoreLoad(t *testing.T) {
	const blobSize = 8 << 10
	const repeatCount = 16

	ctx := context.Background()

	_, store, cleanup := newTestStore(t)
	defer cleanup()

	data := make([]byte, blobSize)
	temp := make([]byte, len(data))
	_, _ = rand.Read(data)

	refs := map[storage.BlobRef]bool{}

	// store without size
	for i := 0; i < repeatCount; i++ {
		ref, err := store.Store(ctx, bytes.NewReader(data), -1)
		if err != nil {
			t.Fatal(err)
		}
		if refs[ref] {
			t.Fatal("duplicate ref received")
		}
		refs[ref] = true
	}

	// store with size
	for i := 0; i < repeatCount; i++ {
		ref, err := store.Store(ctx, bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		if refs[ref] {
			t.Fatal("duplicate ref received")
		}
		refs[ref] = true
	}

	// store with larger size
	{
		ref, err := store.Store(ctx, bytes.NewReader(data), int64(len(data))*2)
		if err != nil {
			t.Fatal(err)
		}
		if refs[ref] {
			t.Fatal("duplicate ref received")
		}
		refs[ref] = true
	}

	// store with error
	{
		_, err := store.Store(ctx, &errorReader{}, int64(len(data)))
		if err == nil {
			t.Fatal("expected store error")
		}
	}

	// try reading all the blobs
	for ref := range refs {
		reader, err := store.Load(ctx, ref)
		if err != nil {
			t.Fatal(err)
		}

		if reader.Size() != int64(len(data)) {
			t.Fatal(err)
		}

		_, err = io.ReadFull(reader, temp)
		if err != nil {
			t.Fatal(err)
		}

		err = reader.Close()
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(data, temp) {
			t.Fatal("data mismatch")
		}
	}

	// delete the blobs
	for ref := range refs {
		err := store.Delete(ctx, ref)
		if err != nil {
			t.Fatal(err)
		}
	}

	// try reading all the blobs
	for ref := range refs {
		_, err := store.Load(ctx, ref)
		if err == nil {
			t.Fatal("expected error when loading invalid ref")
		}
	}
}

func TestDeleteWhileReading(t *testing.T) {
	const blobSize = 8 << 10

	ctx := context.Background()

	dir, store, cleanup := newTestStore(t)
	defer cleanup()

	data := make([]byte, blobSize)
	_, _ = rand.Read(data)

	ref, err := store.Store(ctx, bytes.NewReader(data), -1)
	if err != nil {
		t.Fatal(err)
	}

	rd, loadErr := store.Load(ctx, ref)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	// double close, just in case
	defer func() { _ = rd.Close() }()

	deleteErr := store.Delete(ctx, ref)
	if deleteErr != nil {
		t.Fatal(deleteErr)
	}

	result, readErr := ioutil.ReadAll(rd)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr := rd.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if !bytes.Equal(data, result) {
		t.Fatalf("data mismatch: %v %v", data, result)
	}

	_ = store.GarbageCollect(ctx)

	_, secondLoadErr := store.Load(ctx, ref)
	if !os.IsNotExist(secondLoadErr) {
		t.Fatalf("expected not-exist error got %v", secondLoadErr)
	}

	// flaky test, for checking whether files have been actually deleted from disk
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		return errors.New("found file " + path)
	})
	if err != nil {
		t.Fatal(err)
	}
}

type errorReader struct{}

func (errorReader *errorReader) Read(data []byte) (n int, err error) {
	return 0, errors.New("internal-error")
}

func BenchmarkStoreDelete(b *testing.B) {
	ctx := context.Background()

	var data [8 << 10]byte

	_, store, cleanup := newTestStore(b)
	defer cleanup()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		ref, err := store.Store(ctx, bytes.NewReader(data[:]), int64(len(data)))
		if err != nil {
			b.Fatal(err)
		}
		if err := store.Delete(ctx, ref); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
}
