// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore_test

import (
	"errors"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
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

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, err := filestore.NewAt(ctx.Dir("store"))
	require.NoError(t, err)

	data := make([]byte, blobSize)
	temp := make([]byte, len(data))
	_, _ = rand.Read(data)

	refs := []storage.BlobRef{}

	// store without size
	for i := 0; i < repeatCount; i++ {
		ref := storage.BlobRef{
			Namespace: []byte{0},
			Key:       []byte(strconv.Itoa(i)),
		}
		refs = append(refs, ref)

		writer, err := store.Create(ctx, ref, -1)
		require.NoError(t, err)

		n, err := writer.Write(data)
		require.NoError(t, err)
		require.Equal(t, n, len(data))

		require.NoError(t, writer.Commit())
	}

	// store with size
	for i := 0; i < repeatCount; i++ {
		ref := storage.BlobRef{
			Namespace: []byte{1},
			Key:       []byte(strconv.Itoa(i)),
		}
		refs = append(refs, ref)

		writer, err := store.Create(ctx, ref, int64(len(data)))
		require.NoError(t, err)

		n, err := writer.Write(data)
		require.NoError(t, err)
		require.Equal(t, n, len(data))

		require.NoError(t, writer.Commit())
	}

	// store with larger size
	{
		ref := storage.BlobRef{
			Namespace: []byte{2},
			Key:       []byte{0},
		}
		refs = append(refs, ref)

		writer, err := store.Create(ctx, ref, int64(len(data)*2))
		require.NoError(t, err)

		n, err := writer.Write(data)
		require.NoError(t, err)
		require.Equal(t, n, len(data))

		require.NoError(t, writer.Commit())
	}

	// store with error
	{
		ref := storage.BlobRef{
			Namespace: []byte{3},
			Key:       []byte{0},
		}

		writer, err := store.Create(ctx, ref, -1)
		require.NoError(t, err)

		n, err := writer.Write(data)
		require.NoError(t, err)
		require.Equal(t, n, len(data))

		require.NoError(t, writer.Cancel())

		_, err = store.Open(ctx, ref)
		require.Error(t, err)
	}

	// try reading all the blobs
	for _, ref := range refs {
		reader, err := store.Open(ctx, ref)
		require.NoError(t, err)
		require.Equal(t, reader.Size(), int64(len(data)))

		_, err = io.ReadFull(reader, temp)
		require.NoError(t, err)

		require.NoError(t, reader.Close())
		require.Equal(t, data, temp)
	}

	// delete the blobs
	for _, ref := range refs {
		err := store.Delete(ctx, ref)
		require.NoError(t, err)
	}

	// try reading all the blobs
	for _, ref := range refs {
		_, err := store.Open(ctx, ref)
		require.Error(t, err)
	}
}

func TestDeleteWhileReading(t *testing.T) {
	const blobSize = 8 << 10
	const repeatCount = 16

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, err := filestore.NewAt(ctx.Dir("store"))
	require.NoError(t, err)

	data := make([]byte, blobSize)
	_, _ = rand.Read(data)

	ref := storage.BlobRef{
		Namespace: []byte{0},
		Key:       []byte{1},
	}

	writer, err := store.Create(ctx, ref, -1)
	require.NoError(t, err)

	_, err = writer.Write(data)
	require.NoError(t, err)

	// loading uncommitted file should fail
	_, err = store.Open(ctx, ref)
	require.Error(t, err, "loading uncommitted file should fail")

	// commit the file
	err = writer.Commit()
	require.NoError(t, err, "commit the file")

	// open a reader
	reader, err := store.Open(ctx, ref)
	require.NoError(t, err, "open a reader")

	// double close, just in case
	defer func() { _ = reader.Close() }()

	// delete while reading
	err = store.Delete(ctx, ref)
	require.NoError(t, err, "delete while reading")

	// opening deleted file should fail
	_, err = store.Open(ctx, ref)
	require.Error(t, err, "opening deleted file should fail")

	// read all content
	result, err := ioutil.ReadAll(reader)
	require.NoError(t, err, "read all content")

	// finally close reader
	err = reader.Close()
	require.NoError(t, err)

	// should be able to read the full content
	require.Equal(t, data, result)

	// collect trash
	_ = store.GarbageCollect(ctx)

	// flaky test, for checking whether files have been actually deleted from disk
	err = filepath.Walk(ctx.Dir("store"), func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		return errors.New("found file " + path)
	})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
}
