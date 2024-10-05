// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore_test

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity/testidentity"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
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

	store, err := filestore.NewAt(zaptest.NewLogger(t), ctx.Dir("store"), filestore.DefaultConfig)
	require.NoError(t, err)
	defer ctx.Check(store.Close)

	data := testrand.Bytes(blobSize)

	refs := []blobstore.BlobRef{}

	namespace := testrand.Bytes(32)

	// store without size
	for i := 0; i < repeatCount; i++ {
		ref := blobstore.BlobRef{
			Namespace: namespace,
			Key:       testrand.Bytes(32),
		}
		refs = append(refs, ref)

		writer, err := store.Create(ctx, ref)
		require.NoError(t, err)

		err = writer.ReserveHeader(int64(i))
		require.NoError(t, err)

		n, err := writer.Write(data)
		require.NoError(t, err)
		require.Equal(t, n, len(data))

		// we cannot reserve header after writing data
		err = writer.ReserveHeader(int64(i))
		require.Error(t, err)

		require.NoError(t, writer.Commit(ctx))
		// after committing we should be able to call cancel without an error
		require.NoError(t, writer.Cancel(ctx))
		// two commits should fail
		require.Error(t, writer.Commit(ctx))
	}

	namespace = testrand.Bytes(32)
	// store with error
	{
		ref := blobstore.BlobRef{
			Namespace: namespace,
			Key:       testrand.Bytes(32),
		}

		writer, err := store.Create(ctx, ref)
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
	for i, ref := range refs {
		reader, err := store.Open(ctx, ref)
		require.NoError(t, err)

		size, err := reader.Size()
		require.NoError(t, err)
		// compare to data size and header size
		require.Equal(t, size, int64(len(data)+i))

		temp := make([]byte, len(data)+i)
		_, err = io.ReadFull(reader, temp)
		require.NoError(t, err)

		require.NoError(t, reader.Close())
		require.Equal(t, append(make([]byte, i), data...), temp)
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

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, err := filestore.NewAt(zaptest.NewLogger(t), ctx.Dir("store"), filestore.DefaultConfig)
	require.NoError(t, err)
	defer ctx.Check(store.Close)

	data := testrand.Bytes(blobSize)

	ref := blobstore.BlobRef{
		Namespace: []byte{0},
		Key:       []byte{1},
	}

	writer, err := store.Create(ctx, ref)
	require.NoError(t, err)

	_, err = writer.Write(data)
	require.NoError(t, err)

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
	result, err := io.ReadAll(reader)
	require.NoError(t, err, "read all content")

	// finally close reader
	err = reader.Close()
	require.NoError(t, err)

	// should be able to read the full content
	require.Equal(t, data, result)

	// flaky test, for checking whether files have been actually deleted from disk
	err = filepath.Walk(ctx.Dir("store"), func(path string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}
		if info.Name() == filestore.TrashUsesDayDirsIndicator {
			return nil
		}
		return errs.New("found file %q", path)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeABlob(ctx context.Context, t testing.TB, store blobstore.Blobs, blobRef blobstore.BlobRef, data []byte, formatVersion blobstore.FormatVersion) {
	var (
		blobWriter blobstore.BlobWriter
		err        error
	)
	switch formatVersion {
	case filestore.FormatV0:
		fStore, ok := store.(interface {
			TestCreateV0(ctx context.Context, ref blobstore.BlobRef) (_ blobstore.BlobWriter, err error)
		})
		require.Truef(t, ok, "can't make a WriterForFormatVersion with this blob store (%T)", store)
		blobWriter, err = fStore.TestCreateV0(ctx, blobRef)
	case filestore.FormatV1:
		blobWriter, err = store.Create(ctx, blobRef)
	default:
		t.Fatalf("please teach me how to make a V%d blob", formatVersion)
	}
	require.NoError(t, err)
	require.Equal(t, formatVersion, blobWriter.StorageFormatVersion())
	_, err = blobWriter.Write(data)
	require.NoError(t, err)
	size, err := blobWriter.Size()
	require.NoError(t, err)
	assert.Equal(t, int64(len(data)), size)
	err = blobWriter.Commit(ctx)
	require.NoError(t, err)
}

func verifyBlobHandle(t testing.TB, reader blobstore.BlobReader, expectDataLen int, expectFormat blobstore.FormatVersion) {
	assert.Equal(t, expectFormat, reader.StorageFormatVersion())
	size, err := reader.Size()
	require.NoError(t, err)
	assert.Equal(t, int64(expectDataLen), size)
}

func verifyBlobInfo(ctx context.Context, t testing.TB, blobInfo blobstore.BlobInfo, expectDataLen int, expectFormat blobstore.FormatVersion) {
	assert.Equal(t, expectFormat, blobInfo.StorageFormatVersion())
	stat, err := blobInfo.Stat(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(expectDataLen), stat.Size())
}

func tryOpeningABlob(ctx context.Context, t testing.TB, store blobstore.Blobs, blobRef blobstore.BlobRef, expectDataLen int, expectFormat blobstore.FormatVersion) {
	reader, err := store.Open(ctx, blobRef)
	require.NoError(t, err)
	verifyBlobHandle(t, reader, expectDataLen, expectFormat)
	require.NoError(t, reader.Close())

	blobInfo, err := store.Stat(ctx, blobRef)
	require.NoError(t, err)
	verifyBlobInfo(ctx, t, blobInfo, expectDataLen, expectFormat)

	blobInfo, err = store.StatWithStorageFormat(ctx, blobRef, expectFormat)
	require.NoError(t, err)
	verifyBlobInfo(ctx, t, blobInfo, expectDataLen, expectFormat)

	reader, err = store.OpenWithStorageFormat(ctx, blobInfo.BlobRef(), blobInfo.StorageFormatVersion())
	require.NoError(t, err)
	verifyBlobHandle(t, reader, expectDataLen, expectFormat)
	require.NoError(t, reader.Close())
}

func TestMultipleStorageFormatVersions(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, err := filestore.NewAt(zaptest.NewLogger(t), ctx.Dir("store"), filestore.DefaultConfig)
	require.NoError(t, err)
	defer ctx.Check(store.Close)

	const blobSize = 1024

	var (
		data      = testrand.Bytes(blobSize)
		namespace = testrand.Bytes(namespaceSize)
		v0BlobKey = testrand.Bytes(keySize)
		v1BlobKey = testrand.Bytes(keySize)

		v0Ref = blobstore.BlobRef{Namespace: namespace, Key: v0BlobKey}
		v1Ref = blobstore.BlobRef{Namespace: namespace, Key: v1BlobKey}
	)

	// write a V0 blob
	writeABlob(ctx, t, store, v0Ref, data, filestore.FormatV0)

	// write a V1 blob
	writeABlob(ctx, t, store, v1Ref, data, filestore.FormatV1)

	// look up the different blobs with Open and Stat and OpenWithStorageFormat
	tryOpeningABlob(ctx, t, store, v0Ref, len(data), filestore.FormatV0)
	tryOpeningABlob(ctx, t, store, v1Ref, len(data), filestore.FormatV1)

	// write a V1 blob with the same ID as the V0 blob (to simulate it being rewritten as
	// V1 during a migration), with different data so we can distinguish them
	differentData := make([]byte, len(data)+2)
	copy(differentData, data)
	copy(differentData[len(data):], "\xff\x00")
	writeABlob(ctx, t, store, v0Ref, differentData, filestore.FormatV1)

	// if we try to access the blob at that key, we should see only the V1 blob
	tryOpeningABlob(ctx, t, store, v0Ref, len(differentData), filestore.FormatV1)

	// unless we ask specifically for a V0 blob
	blobInfo, err := store.StatWithStorageFormat(ctx, v0Ref, filestore.FormatV0)
	require.NoError(t, err)
	verifyBlobInfo(ctx, t, blobInfo, len(data), filestore.FormatV0)
	reader, err := store.OpenWithStorageFormat(ctx, blobInfo.BlobRef(), blobInfo.StorageFormatVersion())
	require.NoError(t, err)
	verifyBlobHandle(t, reader, len(data), filestore.FormatV0)
	require.NoError(t, reader.Close())

	// delete the v0BlobKey; both the V0 and the V1 blobs should go away
	err = store.Delete(ctx, v0Ref)
	require.NoError(t, err)

	reader, err = store.Open(ctx, v0Ref)
	require.Error(t, err)
	assert.Nil(t, reader)
}

// Check that the SpaceUsedForBlobs and SpaceUsedForBlobsInNamespace methods on
// filestore.blobStore work as expected.
func TestStoreSpaceUsed(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, err := filestore.NewAt(zaptest.NewLogger(t), ctx.Dir("store"), filestore.DefaultConfig)
	require.NoError(t, err)
	defer ctx.Check(store.Close)

	var (
		namespace      = testrand.Bytes(namespaceSize)
		otherNamespace = testrand.Bytes(namespaceSize)
		sizesToStore   = []memory.Size{4093, 0, 512, 1, memory.MB}
	)

	spaceUsed, err := store.SpaceUsedForBlobs(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), spaceUsed)
	spaceUsed, err = store.SpaceUsedForBlobsInNamespace(ctx, namespace)
	require.NoError(t, err)
	assert.Equal(t, int64(0), spaceUsed)
	spaceUsed, err = store.SpaceUsedForBlobsInNamespace(ctx, otherNamespace)
	require.NoError(t, err)
	assert.Equal(t, int64(0), spaceUsed)

	var totalSoFar memory.Size
	for _, size := range sizesToStore {
		contents := testrand.Bytes(size)
		blobRef := blobstore.BlobRef{Namespace: namespace, Key: testrand.Bytes(keySize)}

		blobWriter, err := store.Create(ctx, blobRef)
		require.NoError(t, err)
		_, err = blobWriter.Write(contents)
		require.NoError(t, err)
		err = blobWriter.Commit(ctx)
		require.NoError(t, err)
		totalSoFar += size

		spaceUsed, err := store.SpaceUsedForBlobs(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(totalSoFar), spaceUsed)
		spaceUsed, err = store.SpaceUsedForBlobsInNamespace(ctx, namespace)
		require.NoError(t, err)
		assert.Equal(t, int64(totalSoFar), spaceUsed)
		spaceUsed, err = store.SpaceUsedForBlobsInNamespace(ctx, otherNamespace)
		require.NoError(t, err)
		assert.Equal(t, int64(0), spaceUsed)
	}
}

// Check that ListNamespaces and WalkNamespace work as expected.
func TestStoreTraversals(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, err := filestore.NewAt(zaptest.NewLogger(t), ctx.Dir("store"), filestore.DefaultConfig)
	require.NoError(t, err)
	defer ctx.Check(store.Close)

	// invent some namespaces and store stuff in them
	type namespaceWithBlobs struct {
		namespace []byte
		blobs     []blobstore.BlobRef
	}
	const numNamespaces = 4
	recordsToInsert := make([]namespaceWithBlobs, numNamespaces)

	var namespaceBase = testrand.Bytes(namespaceSize)
	for i := range recordsToInsert {
		// give each namespace a similar ID but modified in the last byte to distinguish
		recordsToInsert[i].namespace = make([]byte, len(namespaceBase))
		copy(recordsToInsert[i].namespace, namespaceBase)
		recordsToInsert[i].namespace[len(namespaceBase)-1] = byte(i)

		// put varying numbers of blobs in the namespaces
		recordsToInsert[i].blobs = make([]blobstore.BlobRef, i+1)
		for j := range recordsToInsert[i].blobs {
			recordsToInsert[i].blobs[j] = blobstore.BlobRef{
				Namespace: recordsToInsert[i].namespace,
				Key:       testrand.Bytes(keySize),
			}
			blobWriter, err := store.Create(ctx, recordsToInsert[i].blobs[j])
			require.NoError(t, err)
			// also vary the sizes of the blobs so we can check Stat results
			_, err = blobWriter.Write(testrand.Bytes(memory.Size(j)))
			require.NoError(t, err)
			err = blobWriter.Commit(ctx)
			require.NoError(t, err)
		}
	}

	// test ListNamespaces
	gotNamespaces, err := store.ListNamespaces(ctx)
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

	// test WalkNamespace
	for _, expected := range recordsToInsert {
		// this isn't strictly necessary, since the function closure below is not persisted
		// past the end of a loop iteration, but this keeps the linter from complaining.
		expected := expected

		// keep track of which blobs we visit with WalkNamespace
		found := make([]bool, len(expected.blobs))

		err = store.WalkNamespace(ctx, expected.namespace, nil, func(info blobstore.BlobInfo) error {
			gotBlobRef := info.BlobRef()
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
				"WalkNamespace gave BlobRef %v, but I don't remember storing that",
				gotBlobRef)

			// check BlobInfo sanity
			stat, err := info.Stat(ctx)
			require.NoError(t, err)
			assert.Equal(t, int64(blobIdentified), stat.Size())
			return nil
		})
		require.NoError(t, err)

		// make sure all blobs were visited
		for i := range found {
			assert.True(t, found[i],
				"WalkNamespace never yielded blob at index %d: %v",
				i, expected.blobs[i])
		}
	}

	// test WalkNamespace on a nonexistent namespace also
	namespaceBase[len(namespaceBase)-1] = byte(numNamespaces)
	err = store.WalkNamespace(ctx, namespaceBase, nil, func(_ blobstore.BlobInfo) error {
		t.Fatal("this should not have been called")
		return nil
	})
	require.NoError(t, err)

	// check that WalkNamespace stops iterating after an error return
	iterations := 0
	expectedErr := errs.New("an expected error")
	err = store.WalkNamespace(ctx, recordsToInsert[numNamespaces-1].namespace, nil, func(_ blobstore.BlobInfo) error {
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

func TestEmptyTrash(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, err := filestore.NewAt(zaptest.NewLogger(t), ctx.Dir("store"), filestore.DefaultConfig)
	require.NoError(t, err)
	defer ctx.Check(store.Close)

	size := memory.KB

	type testfile struct {
		data      []byte
		formatVer blobstore.FormatVersion
	}
	type testref struct {
		key   []byte
		files []testfile
	}
	type testnamespace struct {
		namespace []byte
		refs      []testref
	}

	namespaces := []testnamespace{
		{
			namespace: testrand.Bytes(namespaceSize),
			refs: []testref{
				{
					// Has v0 and v1
					key: testrand.Bytes(keySize),
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV0,
						},
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV1,
						},
					},
				},
				{
					// Has v0 only
					key: testrand.Bytes(keySize),
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV0,
						},
					},
				},
				{
					// Has v1 only
					key: testrand.Bytes(keySize),
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV0,
						},
					},
				},
			},
		},
		{
			namespace: testrand.Bytes(namespaceSize),
			refs: []testref{
				{
					// Has v1 only
					key: testrand.Bytes(keySize),
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV0,
						},
					},
				},
			},
		},
	}

	for _, namespace := range namespaces {
		for _, ref := range namespace.refs {
			blobref := blobstore.BlobRef{
				Namespace: namespace.namespace,
				Key:       ref.key,
			}

			for _, file := range ref.files {
				var w blobstore.BlobWriter
				if file.formatVer == filestore.FormatV0 {
					fStore, ok := store.(interface {
						TestCreateV0(ctx context.Context, ref blobstore.BlobRef) (_ blobstore.BlobWriter, err error)
					})
					require.Truef(t, ok, "can't make TestCreateV0 with this blob store (%T)", store)
					w, err = fStore.TestCreateV0(ctx, blobref)
				} else if file.formatVer == filestore.FormatV1 {
					w, err = store.Create(ctx, blobref)
				}
				require.NoError(t, err)
				require.NotNil(t, w)
				_, err = w.Write(file.data)
				require.NoError(t, err)

				require.NoError(t, w.Commit(ctx))
				requireFileMatches(ctx, t, store, file.data, blobref, file.formatVer)
			}

			// Trash the ref
			require.NoError(t, store.Trash(ctx, blobref, time.Now()))
		}
	}

	// Restore the first namespace
	var expectedFilesEmptied int64
	for _, ref := range namespaces[0].refs {
		for range ref.files {
			expectedFilesEmptied++
		}
	}
	emptiedBytes, keys, err := store.EmptyTrash(ctx, namespaces[0].namespace, time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, expectedFilesEmptied*int64(size), emptiedBytes)
	assert.Equal(t, int(expectedFilesEmptied), len(keys))
}

func TestTrashAndRestore(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store, err := filestore.NewAt(zaptest.NewLogger(t), ctx.Dir("store"), filestore.DefaultConfig)
	require.NoError(t, err)
	defer ctx.Check(store.Close)

	size := memory.KB

	type testfile struct {
		data      []byte
		formatVer blobstore.FormatVersion
	}
	type testref struct {
		key   []byte
		files []testfile
	}
	type testnamespace struct {
		namespace []byte
		refs      []testref
	}

	namespaces := []testnamespace{
		{
			namespace: testrand.Bytes(namespaceSize),
			refs: []testref{
				{
					// Has v0 and v1
					key: testrand.Bytes(keySize),
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV0,
						},
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV1,
						},
					},
				},
				{
					// Has v0 only
					key: testrand.Bytes(keySize),
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV0,
						},
					},
				},
				{
					// Has v1 only
					key: testrand.Bytes(keySize),
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV0,
						},
					},
				},
			},
		},
		{
			namespace: testrand.Bytes(namespaceSize),
			refs: []testref{
				{
					// Has v1 only
					key: testrand.Bytes(keySize),
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV0,
						},
					},
				},
			},
		},
	}

	for _, namespace := range namespaces {
		for _, ref := range namespace.refs {
			blobref := blobstore.BlobRef{
				Namespace: namespace.namespace,
				Key:       ref.key,
			}

			for _, file := range ref.files {
				var w blobstore.BlobWriter
				if file.formatVer == filestore.FormatV0 {
					fStore, ok := store.(interface {
						TestCreateV0(ctx context.Context, ref blobstore.BlobRef) (_ blobstore.BlobWriter, err error)
					})
					require.Truef(t, ok, "can't make TestCreateV0 with this blob store (%T)", store)
					w, err = fStore.TestCreateV0(ctx, blobref)
				} else if file.formatVer == filestore.FormatV1 {
					w, err = store.Create(ctx, blobref)
				}
				require.NoError(t, err)
				require.NotNil(t, w)
				_, err = w.Write(file.data)
				require.NoError(t, err)

				require.NoError(t, w.Commit(ctx))
				requireFileMatches(ctx, t, store, file.data, blobref, file.formatVer)
			}

			// Trash the ref
			require.NoError(t, store.Trash(ctx, blobref, time.Now()))

			// Verify files are gone
			for n, file := range ref.files {
				_, err = store.OpenWithStorageFormat(ctx, blobref, file.formatVer)
				require.Error(t, err)
				require.True(t, os.IsNotExist(err),
					"on file %d: expected IsNotExist error but got %v", n, err)
			}
		}
	}

	// Restore the first namespace
	var expKeysRestored [][]byte
	for _, ref := range namespaces[0].refs {
		for range ref.files {
			expKeysRestored = append(expKeysRestored, ref.key)
		}
	}
	sort.Slice(expKeysRestored, func(i int, j int) bool {
		return bytes.Compare(expKeysRestored[i], expKeysRestored[j]) < 0
	})
	restoredKeys, err := store.RestoreTrash(ctx, namespaces[0].namespace)
	sort.Slice(restoredKeys, func(i int, j int) bool {
		return bytes.Compare(restoredKeys[i], restoredKeys[j]) < 0
	})
	require.NoError(t, err)
	assert.Equal(t, expKeysRestored, restoredKeys)

	// Verify pieces are back and look good for first namespace
	for _, ref := range namespaces[0].refs {
		blobref := blobstore.BlobRef{
			Namespace: namespaces[0].namespace,
			Key:       ref.key,
		}
		for _, file := range ref.files {
			requireFileMatches(ctx, t, store, file.data, blobref, file.formatVer)
		}
	}

	// Verify pieces in second namespace are still missing (were not restored)
	for _, ref := range namespaces[1].refs {
		blobref := blobstore.BlobRef{
			Namespace: namespaces[1].namespace,
			Key:       ref.key,
		}
		for _, file := range ref.files {
			r, err := store.OpenWithStorageFormat(ctx, blobref, file.formatVer)
			require.Error(t, err)
			require.Nil(t, r)
		}
	}
}

func requireFileMatches(ctx context.Context, t *testing.T, store blobstore.Blobs, data []byte, ref blobstore.BlobRef, formatVer blobstore.FormatVersion) {
	r, err := store.OpenWithStorageFormat(ctx, ref, formatVer)
	require.NoError(t, err)
	defer func() { require.NoError(t, err, r.Close()) }()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)

	require.Equal(t, buf, data)
}

// TestBlobMemoryBuffer ensures that buffering doesn't have problems with
// small writes randomly seeked through the file.
func TestBlobMemoryBuffer(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	const size = 2048

	store, err := filestore.NewAt(zaptest.NewLogger(t), ctx.Dir("store"), filestore.Config{
		WriteBufferSize: 1 * memory.KiB,
	})
	require.NoError(t, err)
	defer ctx.Check(store.Close)

	ref := blobstore.BlobRef{
		Namespace: testrand.Bytes(32),
		Key:       testrand.Bytes(32),
	}

	writer, err := store.Create(ctx, ref)
	require.NoError(t, err)

	for _, v := range rand.Perm(size) {
		_, err := writer.Seek(int64(v), io.SeekStart)
		require.NoError(t, err)
		n, err := writer.Write([]byte{byte(v)})
		require.NoError(t, err)
		require.Equal(t, n, 1)
	}

	_, err = writer.Seek(size, io.SeekStart)
	require.NoError(t, err)

	require.NoError(t, writer.Commit(ctx))

	reader, err := store.Open(ctx, ref)
	require.NoError(t, err)
	defer func() { require.NoError(t, reader.Close()) }()

	buf, err := io.ReadAll(reader)
	require.NoError(t, err)

	for i := range buf {
		require.Equal(t, byte(i), buf[i])
	}
	require.Equal(t, size, len(buf))
}

func TestStorageDirVerification(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	dir, err := filestore.NewDir(log, ctx.Dir("store"))
	require.NoError(t, err)

	ident0, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)

	ident1, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)

	store := filestore.New(log, dir, filestore.Config{
		WriteBufferSize: 1 * memory.KiB,
	})

	// test nonexistent file returns error
	require.Error(t, store.VerifyStorageDir(ctx, ident0.ID))

	// test VerifyWrite doesn't interfere
	require.NoError(t, store.CheckWritability(ctx))
	require.Error(t, store.VerifyStorageDir(ctx, ident0.ID))

	require.NoError(t, dir.CreateVerificationFile(ctx, ident0.ID))

	// test correct ID returns no error
	require.NoError(t, store.VerifyStorageDir(ctx, ident0.ID))

	// test incorrect ID returns error
	err = store.VerifyStorageDir(ctx, ident1.ID)
	require.Contains(t, err.Error(), "does not match running node's ID")

	// test invalid node ID returns error
	f, err := os.Create(filepath.Join(dir.Path(), "storage-dir-verification"))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()
	_, err = f.Write([]byte{0, 1, 2, 3})
	require.NoError(t, err)
	err = store.VerifyStorageDir(ctx, ident0.ID)
	require.Contains(t, err.Error(), "content of file is not a valid node ID")

	// test file overwrite returns no error
	require.NoError(t, dir.CreateVerificationFile(ctx, ident0.ID))
	require.NoError(t, store.VerifyStorageDir(ctx, ident0.ID))
}
