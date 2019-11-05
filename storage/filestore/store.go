// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"context"
	"os"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/storage"
)

var (
	// Error is the default filestore error class
	Error = errs.Class("filestore error")

	mon = monkit.Package()

	_ storage.Blobs = (*Store)(nil)
)

// Store implements a blob store
type Store struct {
	dir *Dir
	log *zap.Logger
}

// New creates a new disk blob store in the specified directory
func New(log *zap.Logger, dir *Dir) *Store {
	return &Store{dir: dir, log: log}
}

// NewAt creates a new disk blob store in the specified directory
func NewAt(log *zap.Logger, path string) (*Store, error) {
	dir, err := NewDir(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &Store{dir: dir, log: log}, nil
}

// Close closes the store.
func (store *Store) Close() error { return nil }

// Open loads blob with the specified hash
func (store *Store) Open(ctx context.Context, ref storage.BlobRef) (_ storage.BlobReader, err error) {
	defer mon.Task()(&ctx)(&err)
	file, formatVer, err := store.dir.Open(ctx, ref)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, Error.Wrap(err)
	}
	return newBlobReader(file, formatVer), nil
}

// OpenWithStorageFormat loads the already-located blob, avoiding the potential need to check multiple
// storage formats to find the blob.
func (store *Store) OpenWithStorageFormat(ctx context.Context, blobRef storage.BlobRef, formatVer storage.FormatVersion) (_ storage.BlobReader, err error) {
	defer mon.Task()(&ctx)(&err)
	file, err := store.dir.OpenWithStorageFormat(ctx, blobRef, formatVer)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, Error.Wrap(err)
	}
	return newBlobReader(file, formatVer), nil
}

// Stat looks up disk metadata on the blob file
func (store *Store) Stat(ctx context.Context, ref storage.BlobRef) (_ storage.BlobInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	info, err := store.dir.Stat(ctx, ref)
	return info, Error.Wrap(err)
}

// StatWithStorageFormat looks up disk metadata on the blob file with the given storage format version
func (store *Store) StatWithStorageFormat(ctx context.Context, ref storage.BlobRef, formatVer storage.FormatVersion) (_ storage.BlobInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	info, err := store.dir.StatWithStorageFormat(ctx, ref, formatVer)
	return info, Error.Wrap(err)
}

// Delete deletes blobs with the specified ref
func (store *Store) Delete(ctx context.Context, ref storage.BlobRef) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.dir.Delete(ctx, ref)
	return Error.Wrap(err)
}

// DeleteWithStorageFormat deletes blobs with the specified ref and storage format version
func (store *Store) DeleteWithStorageFormat(ctx context.Context, ref storage.BlobRef, formatVer storage.FormatVersion) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.dir.DeleteWithStorageFormat(ctx, ref, formatVer)
	return Error.Wrap(err)
}

// GarbageCollect tries to delete any files that haven't yet been deleted
func (store *Store) GarbageCollect(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.dir.GarbageCollect(ctx)
	return Error.Wrap(err)
}

// Create creates a new blob that can be written
// optionally takes a size argument for performance improvements, -1 is unknown size
func (store *Store) Create(ctx context.Context, ref storage.BlobRef, size int64) (_ storage.BlobWriter, err error) {
	defer mon.Task()(&ctx)(&err)
	file, err := store.dir.CreateTemporaryFile(ctx, size)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return newBlobWriter(ref, store, MaxFormatVersionSupported, file), nil
}

// SpaceUsed adds up the space used in all namespaces for blob storage
func (store *Store) SpaceUsed(ctx context.Context) (space int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var totalSpaceUsed int64
	namespaces, err := store.ListNamespaces(ctx)
	if err != nil {
		return 0, Error.New("failed to enumerate namespaces: %v", err)
	}
	for _, namespace := range namespaces {
		used, err := store.SpaceUsedInNamespace(ctx, namespace)
		if err != nil {
			return 0, Error.New("failed to sum space used: %v", err)
		}
		totalSpaceUsed += used
	}
	return totalSpaceUsed, nil
}

// SpaceUsedInNamespace adds up how much is used in the given namespace for blob storage
func (store *Store) SpaceUsedInNamespace(ctx context.Context, namespace []byte) (int64, error) {
	var totalUsed int64
	err := store.WalkNamespace(ctx, namespace, func(info storage.BlobInfo) error {
		statInfo, statErr := info.Stat(ctx)
		if statErr != nil {
			store.log.Error("failed to stat blob", zap.Binary("namespace", namespace), zap.Binary("key", info.BlobRef().Key), zap.Error(statErr))
			// keep iterating; we want a best effort total here.
			return nil
		}
		totalUsed += statInfo.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return totalUsed, nil
}

// FreeSpace returns how much space left in underlying directory
func (store *Store) FreeSpace() (int64, error) {
	info, err := store.dir.Info()
	if err != nil {
		return 0, err
	}
	return info.AvailableSpace, nil
}

// ListNamespaces finds all known namespace IDs in use in local storage. They are not
// guaranteed to contain any blobs.
func (store *Store) ListNamespaces(ctx context.Context) (ids [][]byte, err error) {
	return store.dir.ListNamespaces(ctx)
}

// WalkNamespace executes walkFunc for each locally stored blob in the given namespace. If walkFunc
// returns a non-nil error, WalkNamespace will stop iterating and return the error immediately. The
// ctx parameter is intended specifically to allow canceling iteration early.
func (store *Store) WalkNamespace(ctx context.Context, namespace []byte, walkFunc func(storage.BlobInfo) error) (err error) {
	return store.dir.WalkNamespace(ctx, namespace, walkFunc)
}

// StoreForTest is a wrapper for Store that also allows writing new V0 blobs (in order to test
// situations involving those)
type StoreForTest struct {
	*Store
}

// TestCreateV0 creates a new V0 blob that can be written. This is only appropriate in test situations.
func (store *Store) TestCreateV0(ctx context.Context, ref storage.BlobRef) (_ storage.BlobWriter, err error) {
	defer mon.Task()(&ctx)(&err)

	file, err := store.dir.CreateTemporaryFile(ctx, -1)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return newBlobWriter(ref, store, FormatV0, file), nil
}
