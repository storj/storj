// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"context"
	"encoding/hex"
	"os"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/leak"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/blobstore"
)

var (
	// Error is the default filestore error class.
	Error = errs.Class("filestore error")
	// ErrIsDir is the error returned when we encounter a directory named like a blob file
	// while traversing a blob namespace.
	ErrIsDir = Error.New("file is a directory")

	mon = monkit.Package()
	// for backwards compatibility.
	monStorage = monkit.ScopeNamed("storj.io/storj/storage/filestore")

	_ blobstore.Blobs = (*blobStore)(nil)
)

// MonFileInTrash returns a monkit meter which counts the times a requested blob is
// found in the trash. It is exported so that it can be activated from outside this
// package for backwards compatibility. (It is no longer activated from inside this
// package.)
func MonFileInTrash(namespace []byte) *monkit.Meter {
	return monStorage.Meter("open_file_in_trash", monkit.NewSeriesTag("namespace", hex.EncodeToString(namespace)))
}

// Config is configuration for the blob store.
type Config struct {
	WriteBufferSize memory.Size `help:"in-memory buffer for uploads" default:"128KiB"`
	ForceSync       bool        `help:"if true, force disk synchronization and atomic writes" default:"false"`
}

// DefaultConfig is the default value for Config.
var DefaultConfig = Config{
	WriteBufferSize: 128 * memory.KiB,
	ForceSync:       false,
}

type lazyFile struct {
	ref blobstore.BlobRef
	dir *Dir

	fh *os.File
}

func (f *lazyFile) Write(p []byte) (_ int, err error) {
	if f.fh == nil {
		if err := f.createFile(); err != nil {
			return 0, err
		}
	}
	return f.fh.Write(p)
}

func (f *lazyFile) createFile() (err error) {
	f.fh, err = f.dir.CreateNamedFile(f.ref, MaxFormatVersionSupported)
	return err
}

// blobStore implements a blob store.
type blobStore struct {
	log    *zap.Logger
	dir    *Dir
	config Config

	track leak.Ref
}

// New creates a new disk blob store in the specified directory.
func New(log *zap.Logger, dir *Dir, config Config) blobstore.Blobs {
	return &blobStore{dir: dir, log: log, config: config, track: leak.Root(1)}
}

// NewAt creates a new disk blob store in the specified directory.
func NewAt(log *zap.Logger, path string, config Config) (blobstore.Blobs, error) {
	dir, err := NewDir(log, path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &blobStore{dir: dir, log: log, config: config, track: leak.Root(1)}, nil
}

// OpenAt opens an existing disk blob store in the specified directory.
func OpenAt(log *zap.Logger, path string, config Config) (blobstore.Blobs, error) {
	dir, err := OpenDir(log, path, time.Now())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &blobStore{dir: dir, log: log, config: config, track: leak.Root(1)}, nil
}

// Close closes the store.
func (store *blobStore) Close() error { return store.track.Close() }

var monBlobStoreOpen = mon.Task()

// Open loads blob with the specified hash.
func (store *blobStore) Open(ctx context.Context, ref blobstore.BlobRef) (_ blobstore.BlobReader, err error) {
	defer monBlobStoreOpen(&ctx)(&err)

	file, formatVer, err := store.dir.Open(ctx, ref)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, Error.Wrap(err)
	}
	return newBlobReader(store.track.Child("blobReader", 1), file, formatVer), nil
}

// OpenWithStorageFormat loads the already-located blob, avoiding the potential need to check multiple
// storage formats to find the blob.
func (store *blobStore) OpenWithStorageFormat(ctx context.Context, blobRef blobstore.BlobRef, formatVer blobstore.FormatVersion) (_ blobstore.BlobReader, err error) {
	defer mon.Task()(&ctx)(&err)
	file, err := store.dir.OpenWithStorageFormat(ctx, blobRef, formatVer)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, Error.Wrap(err)
	}
	return newBlobReader(store.track.Child("blobReader", 1), file, formatVer), nil
}

// Stat looks up disk metadata on the blob file.
func (store *blobStore) Stat(ctx context.Context, ref blobstore.BlobRef) (_ blobstore.BlobInfo, err error) {
	// not monkit monitoring because of performance reasons

	info, err := store.dir.Stat(ctx, ref)
	return info, Error.Wrap(err)
}

var monBlobStoreStatWithStorageFormat = mon.Task()

// StatWithStorageFormat looks up disk metadata on the blob file with the given storage format version.
func (store *blobStore) StatWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion) (_ blobstore.BlobInfo, err error) {
	defer monBlobStoreStatWithStorageFormat(&ctx)(&err)
	info, err := store.dir.StatWithStorageFormat(ctx, ref, formatVer)
	return info, Error.Wrap(err)
}

// Delete deletes blobs with the specified ref.
//
// It doesn't return an error if the blob isn't found for any reason.
func (store *blobStore) Delete(ctx context.Context, ref blobstore.BlobRef) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.dir.Delete(ctx, ref)
	return Error.Wrap(err)
}

// DeleteWithStorageFormat deletes blobs with the specified ref and storage format version.
func (store *blobStore) DeleteWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion, sizeHint int64) (err error) {
	// not monkit monitoring because of performance reasons

	err = store.dir.DeleteWithStorageFormat(ctx, ref, formatVer)
	return Error.Wrap(err)
}

// DeleteNamespace deletes blobs folder of specific satellite, used after successful GE only.
func (store *blobStore) DeleteNamespace(ctx context.Context, ref []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.dir.DeleteNamespace(ctx, ref)
	return Error.Wrap(err)
}

// DeleteTrashNamespace deletes trash folder of specific satellite.
func (store *blobStore) DeleteTrashNamespace(ctx context.Context, namespace []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.dir.DeleteTrashNamespace(ctx, namespace)
	return Error.Wrap(err)
}

var monBlobStoreTrash = mon.Task()

// Trash moves the ref to a trash directory.
func (store *blobStore) Trash(ctx context.Context, ref blobstore.BlobRef, timestamp time.Time) (err error) {
	defer monBlobStoreTrash(&ctx)(&err)
	return Error.Wrap(store.dir.Trash(ctx, ref, timestamp))
}

// TrashWithStorageFormat marks a blob with a specific storage format for pending deletion.
func (store *blobStore) TrashWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion, timestamp time.Time) error {
	return Error.Wrap(store.dir.TrashWithStorageFormat(ctx, ref, formatVer, timestamp))
}

// RestoreTrash moves every blob in the trash back into the regular location.
func (store *blobStore) RestoreTrash(ctx context.Context, namespace []byte) (keysRestored [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)
	keysRestored, err = store.dir.RestoreTrash(ctx, namespace)
	return keysRestored, Error.Wrap(err)
}

// TryRestoreTrashBlob attempts to restore a blob from the trash if it exists.
// It returns nil if the blob was restored, or an error if the blob was not
// in the trash or could not be restored.
func (store *blobStore) TryRestoreTrashBlob(ctx context.Context, ref blobstore.BlobRef) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.dir.TryRestoreTrashBlob(ctx, ref)
	if os.IsNotExist(err) {
		return err
	}
	return Error.Wrap(err)
}

// EmptyTrashWithoutStat removes files in trash that have been there since before trashedBefore.
func (store *blobStore) EmptyTrashWithoutStat(ctx context.Context, namespace []byte, trashedBefore time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	return store.dir.EmptyTrashWithoutStat(ctx, namespace, trashedBefore)
}

// EmptyTrash removes files in trash that have been there since before trashedBefore.
func (store *blobStore) EmptyTrash(ctx context.Context, namespace []byte, trashedBefore time.Time) (bytesEmptied int64, keys [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)
	bytesEmptied, keys, err = store.dir.EmptyTrash(ctx, namespace, trashedBefore)
	return bytesEmptied, keys, Error.Wrap(err)
}

// Create creates a new blob that can be written.
func (store *blobStore) Create(ctx context.Context, ref blobstore.BlobRef) (_ blobstore.BlobWriter, err error) {
	defer mon.Task()(&ctx)(&err)
	file := &lazyFile{
		ref: ref,
		dir: store.dir,
	}
	if store.config.ForceSync {
		file.fh, err = store.dir.CreateTemporaryFile(ctx)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	return newBlobWriter(store.track.Child("blobWriter", 1), ref, store, MaxFormatVersionSupported, file, store.config.WriteBufferSize.Int(), store.config.ForceSync), nil
}

// SpaceUsedForBlobs adds up the space used in all namespaces for blob storage.
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

// SpaceUsedForBlobsInNamespace adds up how much is used in the given namespace for blob storage.
func (store *blobStore) SpaceUsedForBlobsInNamespace(ctx context.Context, namespace []byte) (int64, error) {
	var totalUsed int64
	err := store.WalkNamespace(ctx, namespace, nil, func(info blobstore.BlobInfo) error {
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

// SpaceUsedForTrash returns the total space used by the trash.
func (store *blobStore) SpaceUsedForTrash(ctx context.Context) (total int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var totalSpaceUsed int64
	namespaces, err := store.listNamespacesInTrash(ctx)
	if err != nil {
		return 0, Error.New("failed to enumerate namespaces in trash: %w", err)
	}
	for _, namespace := range namespaces {
		used, err := store.SpaceUsedForBlobsInNamespaceInTrash(ctx, namespace)
		if err != nil {
			return 0, Error.New("failed to walk trash namespace %x: %w", namespace, err)
		}
		totalSpaceUsed += used
	}
	return totalSpaceUsed, nil
}

// SpaceUsedForBlobsInNamespaceInTrash adds up how much is used in the given namespace in the trash.
func (store *blobStore) SpaceUsedForBlobsInNamespaceInTrash(ctx context.Context, namespace []byte) (int64, error) {
	var totalUsed int64
	err := store.walkNamespaceInTrash(ctx, namespace, func(info blobstore.BlobInfo, dirTime time.Time) error {
		statInfo, statErr := info.Stat(ctx)
		if statErr != nil {
			store.log.Error("failed to stat blob in trash",
				zap.Binary("namespace", namespace),
				zap.Binary("key", info.BlobRef().Key),
				zap.Error(statErr))
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

// DiskInfo returns information about the disk.
func (store *blobStore) DiskInfo(ctx context.Context) (blobstore.DiskInfo, error) {
	return store.dir.Info(ctx)
}

// CheckWritability tests writability of the storage directory by creating and deleting a file.
func (store *blobStore) CheckWritability(ctx context.Context) error {
	f, err := os.CreateTemp(store.dir.Path(), "write-test")
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Remove(f.Name())
}

// ListNamespaces finds all known namespace IDs in use in local storage. They are not
// guaranteed to contain any blobs.
func (store *blobStore) ListNamespaces(ctx context.Context) (ids [][]byte, err error) {
	return store.dir.ListNamespaces(ctx)
}

// WalkNamespace executes walkFunc for each locally stored blob in the given namespace. If walkFunc
// returns a non-nil error, WalkNamespace will stop iterating and return the error immediately. The
// ctx parameter is intended specifically to allow canceling iteration early.
func (store *blobStore) WalkNamespace(ctx context.Context, namespace []byte, skipPrefixFn blobstore.SkipPrefixFn, walkFunc func(blobstore.BlobInfo) error) (err error) {
	return store.dir.WalkNamespace(ctx, namespace, skipPrefixFn, walkFunc)
}

// walkNamespaceInTrash executes walkFunc for each blob stored in the trash under the given
// namespace. If walkFunc returns a non-nil error, walkNamespaceInTrash will stop iterating and
// return the error immediately. The ctx parameter is intended specifically to allow canceling
// iteration early.
func (store *blobStore) walkNamespaceInTrash(ctx context.Context, namespace []byte, walkFunc func(info blobstore.BlobInfo, dirTime time.Time) error) error {
	return store.dir.walkNamespaceInTrash(ctx, namespace, walkFunc)
}

// TestCreateV0 creates a new V0 blob that can be written. This is ONLY appropriate in test situations.
func (store *blobStore) TestCreateV0(ctx context.Context, ref blobstore.BlobRef) (_ blobstore.BlobWriter, err error) {
	defer mon.Task()(&ctx)(&err)

	file := &lazyFile{
		ref: ref,
		dir: store.dir,
	}
	file.fh, err = store.dir.CreateTemporaryFile(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return newBlobWriter(store.track.Child("blobWriter", 1), ref, store, FormatV0, file, store.config.WriteBufferSize.Int(), store.config.ForceSync), nil
}

// CreateVerificationFile creates a file to be used for storage directory verification.
func (store *blobStore) CreateVerificationFile(ctx context.Context, id storj.NodeID) error {
	return store.dir.CreateVerificationFile(ctx, id)
}

// VerifyStorageDir verifies that the storage directory is correct by checking for the existence and validity
// of the verification file.
func (store *blobStore) VerifyStorageDir(ctx context.Context, id storj.NodeID) error {
	return store.dir.Verify(ctx, id)
}

// listNamespacesInTrash lists all known the namespace IDs in use in the trash. They are
// not guaranteed to contain any blobs, or to correspond to namespaces in main storage.
func (store *blobStore) listNamespacesInTrash(ctx context.Context) ([][]byte, error) {
	return store.dir.listNamespacesInTrash(ctx)
}
