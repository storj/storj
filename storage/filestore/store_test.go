// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore_test

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
)

const (
	namespaceSize = 32
	keySize       = 32
)

func TestStoreLoad(t *testing.T) {
	const blobSize = 8 << 10
	const repeatCount = 16

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, err := filestore.NewAt(ctx.Dir("store"), zaptest.NewLogger(t))
	require.NoError(t, err)

	data := testrand.Bytes(blobSize)
	temp := make([]byte, len(data))

	refs := []storage.BlobRef{}

	namespace := testrand.Bytes(32)

	// store without size
	for i := 0; i < repeatCount; i++ {
		ref := storage.BlobRef{
			Namespace: namespace,
			Key:       testrand.Bytes(32),
		}
		refs = append(refs, ref)

		writer, err := store.Create(ctx, ref, -1)
		require.NoError(t, err)

		n, err := writer.Write(data)
		require.NoError(t, err)
		require.Equal(t, n, len(data))

		require.NoError(t, writer.Commit(ctx))
		// after committing we should be able to call cancel without an error
		require.NoError(t, writer.Cancel(ctx))
		// two commits should fail
		require.Error(t, writer.Commit(ctx))
	}

	namespace = testrand.Bytes(32)
	// store with size
	for i := 0; i < repeatCount; i++ {
		ref := storage.BlobRef{
			Namespace: namespace,
			Key:       testrand.Bytes(32),
		}
		refs = append(refs, ref)

		writer, err := store.Create(ctx, ref, int64(len(data)))
		require.NoError(t, err)

		n, err := writer.Write(data)
		require.NoError(t, err)
		require.Equal(t, n, len(data))

		require.NoError(t, writer.Commit(ctx))
	}

	namespace = testrand.Bytes(32)
	// store with larger size
	{
		ref := storage.BlobRef{
			Namespace: namespace,
			Key:       testrand.Bytes(32),
		}
		refs = append(refs, ref)

		writer, err := store.Create(ctx, ref, int64(len(data)*2))
		require.NoError(t, err)

		n, err := writer.Write(data)
		require.NoError(t, err)
		require.Equal(t, n, len(data))

		require.NoError(t, writer.Commit(ctx))
	}

	namespace = testrand.Bytes(32)
	// store with error
	{
		ref := storage.BlobRef{
			Namespace: namespace,
			Key:       testrand.Bytes(32),
		}

		writer, err := store.Create(ctx, ref, -1)
		require.NoError(t, err)

		n, err := writer.Write(data)
		require.NoError(t, err)
		require.Equal(t, n, len(data))

		require.NoError(t, writer.Cancel(ctx))
		// commit after cancel should return an error
		require.Error(t, writer.Commit(ctx))

		_, err = store.Open(ctx, ref)
		require.Error(t, err)
	}

	// try reading all the blobs
	for _, ref := range refs {
		reader, err := store.Open(ctx, ref)
		require.NoError(t, err)

		size, err := reader.Size()
		require.NoError(t, err)
		require.Equal(t, size, int64(len(data)))

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

	store, err := filestore.NewAt(ctx.Dir("store"), zaptest.NewLogger(t))
	require.NoError(t, err)

	data := testrand.Bytes(blobSize)

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
	err = writer.Commit(ctx)
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
		return errs.New("found file %q", path)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeABlob(ctx context.Context, t testing.TB, store *filestore.Store, blobRef storage.BlobRef, data []byte, formatVersion storage.FormatVersion) {
	var (
		blobWriter storage.BlobWriter
		err        error
	)
	switch formatVersion {
	case storage.FormatV0:
		tStore := &filestore.StoreForTest{store}
		blobWriter, err = tStore.CreateV0(ctx, blobRef)
	case storage.FormatV1:
		blobWriter, err = store.Create(ctx, blobRef, int64(len(data)))
	default:
		t.Fatalf("please teach me how to make a V%d blob", formatVersion)
	}
	require.NoError(t, err)
	require.Equal(t, formatVersion, blobWriter.GetStorageFormatVersion())
	_, err = blobWriter.Write(data)
	require.NoError(t, err)
	size, err := blobWriter.Size()
	require.NoError(t, err)
	assert.Equal(t, int64(len(data)), size)
	err = blobWriter.Commit(ctx)
	require.NoError(t, err)
}

func verifyBlobHandle(t testing.TB, reader storage.BlobReader, expectDataLen int, expectFormat storage.FormatVersion) {
	assert.Equal(t, expectFormat, reader.GetStorageFormatVersion())
	size, err := reader.Size()
	require.NoError(t, err)
	assert.Equal(t, int64(expectDataLen), size)
}

func verifyBlobAccess(ctx context.Context, t testing.TB, blobAccess storage.StoredBlobAccess, expectDataLen int, expectFormat storage.FormatVersion) {
	assert.Equal(t, expectFormat, blobAccess.StorageFormatVersion())
	stat, err := blobAccess.Stat(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(expectDataLen), stat.Size())
}

func tryOpeningABlob(ctx context.Context, t testing.TB, store *filestore.Store, blobRef storage.BlobRef, expectDataLen int, expectFormat storage.FormatVersion) {
	reader, err := store.Open(ctx, blobRef)
	require.NoError(t, err)
	verifyBlobHandle(t, reader, expectDataLen, expectFormat)
	require.NoError(t, reader.Close())

	blobAccess, err := store.Lookup(ctx, blobRef)
	require.NoError(t, err)
	verifyBlobAccess(ctx, t, blobAccess, expectDataLen, expectFormat)

	blobAccess, err = store.LookupSpecific(ctx, blobRef, expectFormat)
	require.NoError(t, err)
	verifyBlobAccess(ctx, t, blobAccess, expectDataLen, expectFormat)

	reader, err = store.OpenLocated(ctx, blobAccess)
	require.NoError(t, err)
	verifyBlobHandle(t, reader, expectDataLen, expectFormat)
	require.NoError(t, reader.Close())
}

func TestMultipleStorageFormatVersions(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, err := filestore.NewAt(ctx.Dir("store"), zaptest.NewLogger(t))
	require.NoError(t, err)

	const blobSize = 1024

	var (
		data      = testrand.Bytes(blobSize)
		namespace = testrand.Bytes(namespaceSize)
		v0BlobKey = testrand.Bytes(keySize)
		v1BlobKey = testrand.Bytes(keySize)

		v0Ref = storage.BlobRef{Namespace: namespace, Key: v0BlobKey}
		v1Ref = storage.BlobRef{Namespace: namespace, Key: v1BlobKey}
	)

	// write a V0 blob
	writeABlob(ctx, t, store, v0Ref, data, storage.FormatV0)

	// write a V1 blob
	writeABlob(ctx, t, store, v1Ref, data, storage.FormatV1)

	// look up the different blobs with Open and Lookup and OpenLocated
	tryOpeningABlob(ctx, t, store, v0Ref, len(data), storage.FormatV0)
	tryOpeningABlob(ctx, t, store, v1Ref, len(data), storage.FormatV1)

	// write a V1 blob with the same ID as the V0 blob (to simulate it being rewritten as
	// V1 during a migration)
	differentData := append(data, 255, 24)
	writeABlob(ctx, t, store, v0Ref, differentData, storage.FormatV1)

	// if we try to access the blob at that key, we should see only the V1 blob
	tryOpeningABlob(ctx, t, store, v0Ref, len(differentData), storage.FormatV1)

	// unless we ask specifically for a V0 blob
	blobAccess, err := store.LookupSpecific(ctx, v0Ref, storage.FormatV0)
	verifyBlobAccess(ctx, t, blobAccess, len(data), storage.FormatV0)
	reader, err := store.OpenLocated(ctx, blobAccess)
	require.NoError(t, err)
	verifyBlobHandle(t, reader, len(data), storage.FormatV0)
	require.NoError(t, reader.Close())

	// delete the v0BlobKey; both the V0 and the V1 blobs should go away
	err = store.Delete(ctx, v0Ref)
	require.NoError(t, err)

	reader, err = store.Open(ctx, v0Ref)
	require.Error(t, err)
	assert.Nil(t, reader)
}

// Check that the SpaceUsed and SpaceUsedInNamespace methods on filestore.Store
// work as expected.
func TestStoreSpaceUsed(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, err := filestore.NewAt(ctx.Dir("store"), zaptest.NewLogger(t))
	require.NoError(t, err)

	var (
		namespaceBase  = testrand.Bytes(namespaceSize - 1)
		namespace      = append(namespaceBase, 0)
		otherNamespace = append(namespaceBase, 1)
		sizesToStore   = []memory.Size{4093, 0, 512, 1, memory.MB}
	)

	spaceUsed, err := store.SpaceUsed(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), spaceUsed)
	spaceUsed, err = store.SpaceUsedInNamespace(ctx, namespace)
	require.NoError(t, err)
	assert.Equal(t, int64(0), spaceUsed)
	spaceUsed, err = store.SpaceUsedInNamespace(ctx, otherNamespace)
	require.NoError(t, err)
	assert.Equal(t, int64(0), spaceUsed)

	var totalSoFar memory.Size
	for _, size := range sizesToStore {
		contents := testrand.Bytes(size)
		blobRef := storage.BlobRef{Namespace: namespace, Key: testrand.Bytes(keySize)}

		blobWriter, err := store.Create(ctx, blobRef, int64(len(contents)))
		require.NoError(t, err)
		_, err = blobWriter.Write(contents)
		require.NoError(t, err)
		err = blobWriter.Commit(ctx)
		require.NoError(t, err)
		totalSoFar += size

		spaceUsed, err := store.SpaceUsed(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(totalSoFar), spaceUsed)
		spaceUsed, err = store.SpaceUsedInNamespace(ctx, namespace)
		require.NoError(t, err)
		assert.Equal(t, int64(totalSoFar), spaceUsed)
		spaceUsed, err = store.SpaceUsedInNamespace(ctx, otherNamespace)
		require.NoError(t, err)
		assert.Equal(t, int64(0), spaceUsed)
	}
}

// Check that GetAllNamespaces and ForAllKeysInNamespace work as expected.
func TestStoreTraversals(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, err := filestore.NewAt(ctx.Dir("store"), zaptest.NewLogger(t))
	require.NoError(t, err)

	// invent some namespaces and store stuff in them
	type namespaceWithBlobs struct {
		namespace []byte
		blobs     []storage.BlobRef
	}
	const numNamespaces = 4
	recordsToInsert := make([]namespaceWithBlobs, numNamespaces)

	var namespaceBase = testrand.Bytes(namespaceSize - 1)
	for i := range recordsToInsert {
		recordsToInsert[i].namespace = append(namespaceBase, byte(i))

		// put varying numbers of blobs in the namespaces
		recordsToInsert[i].blobs = make([]storage.BlobRef, i+1)
		for j := range recordsToInsert[i].blobs {
			recordsToInsert[i].blobs[j] = storage.BlobRef{
				Namespace: recordsToInsert[i].namespace,
				Key:       testrand.Bytes(keySize),
			}
			blobWriter, err := store.Create(ctx, recordsToInsert[i].blobs[j], 0)
			require.NoError(t, err)
			// also vary the sizes of the blobs so we can check Stat results
			_, err = blobWriter.Write(testrand.Bytes(memory.Size(j)))
			require.NoError(t, err)
			err = blobWriter.Commit(ctx)
			require.NoError(t, err)
		}
	}

	// test GetAllNamespaces
	gotNamespaces, err := store.GetAllNamespaces(ctx)
	require.NoError(t, err)
	sort.Slice(gotNamespaces, func(i, j int) bool {
		return bytes.Compare(gotNamespaces[i], gotNamespaces[j]) < 0
	})
	sort.Slice(recordsToInsert, func(i, j int) bool {
		return bytes.Compare(recordsToInsert[i].namespace, recordsToInsert[j].namespace) < 0
	})
	for i, expected := range recordsToInsert {
		require.Equalf(t, expected.namespace, gotNamespaces[i], "mismatch at index %d: recordsToInsert is %+v and gotNamespaces is %v", i, recordsToInsert, gotNamespaces)
	}

	// test ForAllKeysInNamespace
	for _, expected := range recordsToInsert {
		// keep track of which blobs we visit with ForAllKeysInNamespace
		found := make([]bool, len(expected.blobs))

		err = store.ForAllKeysInNamespace(ctx, expected.namespace, func(access storage.StoredBlobAccess) error {
			gotBlobRef := access.BlobRef()
			assert.Equal(t, expected.namespace, gotBlobRef.Namespace)
			// find which blob this is in expected.blobs
			blobIdentified := -1
			for i, expectedBlobRef := range expected.blobs {
				if bytes.Equal(gotBlobRef.Key, expectedBlobRef.Key) {
					found[i] = true
					blobIdentified = i
				}
			}
			// make sure this is a blob we actually put in
			require.NotEqualf(t, -1, blobIdentified,
				"ForAllKeysInNamespace gave BlobRef %v, but I don't remember storing that",
				gotBlobRef)

			// check StoredBlobAccess sanity
			stat, err := access.Stat(ctx)
			require.NoError(t, err)
			nameFromStat := stat.Name()
			fullPath, err := access.FullPath(ctx)
			require.NoError(t, err)
			basePath := filepath.Base(fullPath)
			assert.Equal(t, nameFromStat, basePath)
			assert.Equal(t, int64(blobIdentified), stat.Size())
			assert.False(t, stat.IsDir())
			return nil
		})
		require.NoError(t, err)

		// make sure all blobs were visited
		for i := range found {
			assert.True(t, found[i],
				"ForAllKeysInNamespace never yielded blob at index %d: %v",
				i, expected.blobs[i])
		}
	}

	// test ForAllKeysInNamespace on a nonexistent namespace also
	err = store.ForAllKeysInNamespace(ctx, append(namespaceBase, byte(numNamespaces)), func(access storage.StoredBlobAccess) error {
		t.Fatal("this should not have been called")
		return nil
	})
	require.NoError(t, err)

	// check that ForAllKeysInNamespace stops iterating after an error return
	iterations := 0
	expectedErr := errs.New("an expected error")
	err = store.ForAllKeysInNamespace(ctx, recordsToInsert[numNamespaces-1].namespace, func(access storage.StoredBlobAccess) error {
		iterations++
		if iterations == 2 {
			return expectedErr
		}
		return nil
	})
	assert.Error(t, err)
	assert.Equal(t, err, expectedErr)
	assert.Equal(t, 2, iterations)
}
