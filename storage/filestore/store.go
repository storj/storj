// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/storj/storage"
)

var (
	// Error is the default filestore error class
	Error = errs.Class("filestore error")

	mon            = monkit.Package()
	monFileInTrash = mon.Meter("open_file_in_trash") //locked

	_ storage.Blobs = (*blobStore)(nil)
)

// Config is configuration for the blob store.
type Config struct {
	WriteBufferSize memory.Size `help:"in-memory buffer for uploads" default:"128KiB"`
}

// DefaultConfig is the default value for Config.
var DefaultConfig = Config{
	WriteBufferSize: 128 * memory.KiB,
}

// blobStore implements a blob store
type blobStore struct {
	log    *zap.Logger
	dir    *Dir
	config Config
}

// New creates a new disk blob store in the specified directory
func New(log *zap.Logger, dir *Dir, config Config) storage.Blobs {
	return &blobStore{dir: dir, log: log, config: config}
}

// NewAt creates a new disk blob store in the specified directory
func NewAt(log *zap.Logger, path string, config Config) (storage.Blobs, error) {
	dir, err := NewDir(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &blobStore{dir: dir, log: log, config: config}, nil
}

// Close closes the store.
func (store *blobStore) Close() error { return nil }

// Open loads blob with the specified hash
func (store *blobStore) Open(ctx context.Context, ref storage.BlobRef) (_ storage.BlobReader, err error) {
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
func (store *blobStore) OpenWithStorageFormat(ctx context.Context, blobRef storage.BlobRef, formatVer storage.FormatVersion) (_ storage.BlobReader, err error) {
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
func (store *blobStore) Stat(ctx context.Context, ref storage.BlobRef) (_ storage.BlobInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	info, err := store.dir.Stat(ctx, ref)
	return info, Error.Wrap(err)
}

// StatWithStorageFormat looks up disk metadata on the blob file with the given storage format version
func (store *blobStore) StatWithStorageFormat(ctx context.Context, ref storage.BlobRef, formatVer storage.FormatVersion) (_ storage.BlobInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	info, err := store.dir.StatWithStorageFormat(ctx, ref, formatVer)
	return info, Error.Wrap(err)
}

// Delete deletes blobs with the specified ref.
//
// It doesn't return an error if the blob isn't found for any reason or it cannot
// be deleted at this moment and it's delayed.
func (store *blobStore) Delete(ctx context.Context, ref storage.BlobRef) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.dir.Delete(ctx, ref)
	return Error.Wrap(err)
}

// DeleteWithStorageFormat deletes blobs with the specified ref and storage format version
func (store *blobStore) DeleteWithStorageFormat(ctx context.Context, ref storage.BlobRef, formatVer storage.FormatVersion) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.dir.DeleteWithStorageFormat(ctx, ref, formatVer)
	return Error.Wrap(err)
}

// Trash moves the ref to a trash directory
func (store *blobStore) Trash(ctx context.Context, ref storage.BlobRef) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.Wrap(store.dir.Trash(ctx, ref))
}

// RestoreTrash moves every piece in the trash back into the regular location
func (store *blobStore) RestoreTrash(ctx context.Context, namespace []byte) (keysRestored [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)
	keysRestored, err = store.dir.RestoreTrash(ctx, namespace)
	return keysRestored, Error.Wrap(err)
}

// // EmptyTrash removes all files in trash that have been there longer than trashExpiryDur
func (store *blobStore) EmptyTrash(ctx context.Context, namespace []byte, trashedBefore time.Time) (bytesEmptied int64, keys [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)
	bytesEmptied, keys, err = store.dir.EmptyTrash(ctx, namespace, trashedBefore)
	return bytesEmptied, keys, Error.Wrap(err)
}

// GarbageCollect tries to delete any files that haven't yet been deleted
func (store *blobStore) GarbageCollect(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.dir.GarbageCollect(ctx)
	return Error.Wrap(err)
}

// Create creates a new blob that can be written
// optionally takes a size argument for performance improvements, -1 is unknown size
func (store *blobStore) Create(ctx context.Context, ref storage.BlobRef, size int64) (_ storage.BlobWriter, err error) {
	defer mon.Task()(&ctx)(&err)
	file, err := store.dir.CreateTemporaryFile(ctx, size)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return newBlobWriter(ref, store, MaxFormatVersionSupported, file, store.config.WriteBufferSize.Int()), nil
}

// SpaceUsedForBlobs adds up the space used in all namespaces for blob storage
func (store *blobStore) SpaceUsedForBlobs(ctx context.Context) (space int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var totalSpaceUsed int64
	namespaces, err := store.ListNamespaces(ctx)
	if err != nil {
		return 0, Error.New("failed to enumerate namespaces: %v", err)
	}
	for _, namespace := range namespaces {
		used, err := store.SpaceUsedForBlobsInNamespace(ctx, namespace)
		if err != nil {
			return 0, Error.New("failed to sum space used: %v", err)
		}
		totalSpaceUsed += used
	}
	return totalSpaceUsed, nil
}

// SpaceUsedForBlobsInNamespace adds up how much is used in the given namespace for blob storage
func (store *blobStore) SpaceUsedForBlobsInNamespace(ctx context.Context, namespace []byte) (int64, error) {
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

// TrashIsEmpty returns boolean value if trash dir is empty.
func (store *blobStore) TrashIsEmpty() (_ bool, err error) {
	f, err := os.Open(store.dir.trashdir())
	if err != nil {
		return false, err
	}
	defer func() {
		err = errs.Combine(err, f.Close())
	}()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// SpaceUsedForTrash returns the total space used by the trash
func (store *blobStore) SpaceUsedForTrash(ctx context.Context) (total int64, err error) {
	defer mon.Task()(&ctx)(&err)

	empty, err := store.TrashIsEmpty()
	if err != nil {
		return total, err
	}
	if empty {
		return 0, nil
	}
	err = filepath.Walk(store.dir.trashdir(), func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			err = errs.Combine(err, walkErr)
			return filepath.SkipDir
		}

		total += info.Size()
		return nil
	})
	return total, err
}

// FreeSpace returns how much space left in underlying directory
func (store *blobStore) FreeSpace() (int64, error) {
	info, err := store.dir.Info()
	if err != nil {
		return 0, err
	}
	return info.AvailableSpace, nil
}

// ListNamespaces finds all known namespace IDs in use in local storage. They are not
// guaranteed to contain any blobs.
func (store *blobStore) ListNamespaces(ctx context.Context) (ids [][]byte, err error) {
	return store.dir.ListNamespaces(ctx)
}

// WalkNamespace executes walkFunc for each locally stored blob in the given namespace. If walkFunc
// returns a non-nil error, WalkNamespace will stop iterating and return the error immediately. The
// ctx parameter is intended specifically to allow canceling iteration early.
func (store *blobStore) WalkNamespace(ctx context.Context, namespace []byte, walkFunc func(storage.BlobInfo) error) (err error) {
	return store.dir.WalkNamespace(ctx, namespace, walkFunc)
}

// TestCreateV0 creates a new V0 blob that can be written. This is ONLY appropriate in test situations.
func (store *blobStore) TestCreateV0(ctx context.Context, ref storage.BlobRef) (_ storage.BlobWriter, err error) {
	defer mon.Task()(&ctx)(&err)

	file, err := store.dir.CreateTemporaryFile(ctx, -1)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return newBlobWriter(ref, store, FormatV0, file, store.config.WriteBufferSize.Int()), nil
}
