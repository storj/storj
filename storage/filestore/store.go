// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"context"
	"os"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

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

	spaceReserved int64
}

// New creates a new disk blob store in the specified directory
func New(dir *Dir, log *zap.Logger) *Store {
	return &Store{dir: dir, log: log}
}

// NewAt creates a new disk blob store in the specified directory
func NewAt(path string, log *zap.Logger) (*Store, error) {
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

// OpenLocated loads the already-located blob, avoiding the potential need to check multiple
// storage formats to find the blob.
func (store *Store) OpenLocated(ctx context.Context, access storage.StoredBlobAccess) (_ storage.BlobReader, err error) {
	defer mon.Task()(&ctx)(&err)
	file, err := store.dir.OpenLocated(ctx, access)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, Error.Wrap(err)
	}
	return newBlobReader(file, access.StorageFormatVersion()), nil
}

// Lookup looks up disk metadata on the blob file
func (store *Store) Lookup(ctx context.Context, ref storage.BlobRef) (_ storage.StoredBlobAccess, err error) {
	defer mon.Task()(&ctx)(&err)
	access, err := store.dir.Lookup(ctx, ref)
	return access, Error.Wrap(err)
}

// LookupSpecific looks up disk metadata on the blob file with the given storage format version
func (store *Store) LookupSpecific(ctx context.Context, ref storage.BlobRef, formatVer storage.FormatVersion) (_ storage.StoredBlobAccess, err error) {
	defer mon.Task()(&ctx)(&err)
	access, err := store.dir.LookupSpecific(ctx, ref, formatVer)
	return access, Error.Wrap(err)
}

// Delete deletes blobs with the specified ref
func (store *Store) Delete(ctx context.Context, ref storage.BlobRef) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.dir.Delete(ctx, ref)
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
	return newBlobWriter(ref, store, file), nil
}

// SpaceUsed adds up the space used in all namespaces
func (store *Store) SpaceUsed(ctx context.Context) (space int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var totalSpaceUsed int64
	namespaces, err := store.GetAllNamespaces(ctx)
	if err != nil {
		return 0, err
	}
	for _, namespace := range namespaces {
		used, err := store.SpaceUsedInNamespace(ctx, namespace)
		if err != nil {
			return 0, err
		}
		totalSpaceUsed += used
	}
	return totalSpaceUsed, nil
}

// SpaceUsedInNamespace adds up how much is used in the given namespace
func (store *Store) SpaceUsedInNamespace(ctx context.Context, namespace []byte) (int64, error) {
	var totalUsed int64
	err := store.ForAllV1KeysInNamespace(ctx, namespace, time.Now(), func(access storage.StoredBlobAccess) error {
		statInfo, statErr := access.Stat(ctx)
		if statErr != nil {
			store.log.Sugar().Errorf("failed to stat: %v", statErr)
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
	return info.AvailableSpace - store.spaceReserved, nil
}

// ReserveSpace marks some amount of space as used (although it's not); this only causes FreeSpace()
// to return a lesser amount.
func (store *Store) ReserveSpace(amount int64) {
	store.spaceReserved = amount
}

// GetAllNamespaces finds all known namespace IDs in use in local storage. They are not
// guaranteed to contain any blobs.
func (store *Store) GetAllNamespaces(ctx context.Context) (ids [][]byte, err error) {
	return store.dir.GetAllNamespaces(ctx)
}

// ForAllV1KeysInNamespace executes doForEach for each locally stored blob, stored with
// storage format V1 or greater, in the given namespace, if that blob was created before the
// specified time. If doForEach returns a non-nil error, ForAllKeysInNamespace will stop
// iterating and return the error immediately.
func (store *Store) ForAllV1KeysInNamespace(ctx context.Context, namespace []byte, createdBefore time.Time, doForEach func(storage.StoredBlobAccess) error) (err error) {
	return store.dir.ForAllV1KeysInNamespace(ctx, namespace, createdBefore, doForEach)
}
