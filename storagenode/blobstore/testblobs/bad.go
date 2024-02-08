// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testblobs

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/blobstore"
)

// ErrorBlobs is the interface of blobstore.Blobs with the SetError method added.
// This allows the BadDB{}.Blobs member to be replaced with something that has
// specific behavior changes.
type ErrorBlobs interface {
	blobstore.Blobs
	SetError(err error)
	SetCheckError(err error)
}

// BadDB implements bad storage node DB.
type BadDB struct {
	storagenode.DB
	Blobs ErrorBlobs
	log   *zap.Logger
}

// NewBadDB creates a new bad storage node DB.
// Use SetError to manually configure the error returned by all blob operations.
func NewBadDB(log *zap.Logger, db storagenode.DB) *BadDB {
	return &BadDB{
		DB:    db,
		Blobs: newBadBlobs(log, db.Pieces()),
		log:   log,
	}
}

// Pieces returns the blob store.
func (bad *BadDB) Pieces() blobstore.Blobs {
	return bad.Blobs
}

// SetError sets an error to be returned for blob operations.
func (bad *BadDB) SetError(err error) {
	bad.Blobs.SetError(err)
}

// SetCheckError sets an error to be returned for check and verification operations.
func (bad *BadDB) SetCheckError(err error) {
	bad.Blobs.SetCheckError(err)
}

// BadBlobs implements a bad blob store.
type BadBlobs struct {
	err      lockedErr
	checkErr lockedErr
	blobs    blobstore.Blobs
	log      *zap.Logger
}

type lockedErr struct {
	mu  sync.Mutex
	err error
}

// Err returns the error.
func (m *lockedErr) Err() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.err
}

// Set sets the error.
func (m *lockedErr) Set(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// newBadBlobs creates a new bad blob store wrapping the provided blobs.
// Use SetError to manually configure the error returned by all operations.
func newBadBlobs(log *zap.Logger, blobs blobstore.Blobs) *BadBlobs {
	return &BadBlobs{
		log:   log,
		blobs: blobs,
	}
}

// SetError configures the blob store to return a specific error for all operations, except verification.
func (bad *BadBlobs) SetError(err error) {
	bad.err.Set(err)
}

// SetCheckError configures the blob store to return a specific error for verification operations.
func (bad *BadBlobs) SetCheckError(err error) {
	bad.checkErr.Set(err)
}

// Create creates a new blob that can be written optionally takes a size
// argument for performance improvements, -1 is unknown size.
func (bad *BadBlobs) Create(ctx context.Context, ref blobstore.BlobRef, size int64) (blobstore.BlobWriter, error) {
	if err := bad.err.Err(); err != nil {
		return nil, err
	}
	return bad.blobs.Create(ctx, ref, size)
}

// Close closes the blob store and any resources associated with it.
func (bad *BadBlobs) Close() error {
	if err := bad.err.Err(); err != nil {
		return err
	}
	return bad.blobs.Close()
}

// Open opens a reader with the specified namespace and key.
func (bad *BadBlobs) Open(ctx context.Context, ref blobstore.BlobRef) (blobstore.BlobReader, error) {
	if err := bad.err.Err(); err != nil {
		return nil, err
	}
	return bad.blobs.Open(ctx, ref)
}

// OpenWithStorageFormat opens a reader for the already-located blob, avoiding the potential need
// to check multiple storage formats to find the blob.
func (bad *BadBlobs) OpenWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion) (blobstore.BlobReader, error) {
	if err := bad.err.Err(); err != nil {
		return nil, err
	}
	return bad.blobs.OpenWithStorageFormat(ctx, ref, formatVer)
}

// Trash deletes the blob with the namespace and key.
func (bad *BadBlobs) Trash(ctx context.Context, ref blobstore.BlobRef) error {
	if err := bad.err.Err(); err != nil {
		return err
	}
	return bad.blobs.Trash(ctx, ref)
}

// RestoreTrash restores all files in the trash.
func (bad *BadBlobs) RestoreTrash(ctx context.Context, namespace []byte) ([][]byte, error) {
	if err := bad.err.Err(); err != nil {
		return nil, err
	}
	return bad.blobs.RestoreTrash(ctx, namespace)
}

// EmptyTrash empties the trash.
func (bad *BadBlobs) EmptyTrash(ctx context.Context, namespace []byte, trashedBefore time.Time) (int64, [][]byte, error) {
	if err := bad.err.Err(); err != nil {
		return 0, nil, err
	}
	return bad.blobs.EmptyTrash(ctx, namespace, trashedBefore)
}

// TryRestoreTrashBlob attempts to restore a blob from the trash.
func (bad *BadBlobs) TryRestoreTrashBlob(ctx context.Context, ref blobstore.BlobRef) error {
	if err := bad.err.Err(); err != nil {
		return err
	}
	return bad.blobs.TryRestoreTrashBlob(ctx, ref)
}

// Delete deletes the blob with the namespace and key.
func (bad *BadBlobs) Delete(ctx context.Context, ref blobstore.BlobRef) error {
	if err := bad.err.Err(); err != nil {
		return err
	}
	return bad.blobs.Delete(ctx, ref)
}

// DeleteWithStorageFormat deletes the blob with the namespace, key, and format version.
func (bad *BadBlobs) DeleteWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion) error {
	if err := bad.err.Err(); err != nil {
		return err
	}
	return bad.blobs.DeleteWithStorageFormat(ctx, ref, formatVer)
}

// DeleteNamespace deletes blobs of specific satellite, used after successful GE only.
func (bad *BadBlobs) DeleteNamespace(ctx context.Context, ref []byte) (err error) {
	if err := bad.err.Err(); err != nil {
		return err
	}
	return bad.blobs.DeleteNamespace(ctx, ref)
}

// DeleteTrashNamespace deletes the trash folder for the namespace.
func (bad *BadBlobs) DeleteTrashNamespace(ctx context.Context, namespace []byte) error {
	if err := bad.err.Err(); err != nil {
		return err
	}
	return bad.blobs.DeleteTrashNamespace(ctx, namespace)
}

// Stat looks up disk metadata on the blob file.
func (bad *BadBlobs) Stat(ctx context.Context, ref blobstore.BlobRef) (blobstore.BlobInfo, error) {
	if err := bad.err.Err(); err != nil {
		return nil, err
	}
	return bad.blobs.Stat(ctx, ref)
}

// StatWithStorageFormat looks up disk metadata for the blob file with the given storage format
// version. This avoids the potential need to check multiple storage formats for the blob
// when the format is already known.
func (bad *BadBlobs) StatWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion) (blobstore.BlobInfo, error) {
	if err := bad.err.Err(); err != nil {
		return nil, err
	}
	return bad.blobs.StatWithStorageFormat(ctx, ref, formatVer)
}

// WalkNamespace executes walkFunc for each locally stored blob in the given namespace.
// If walkFunc returns a non-nil error, WalkNamespace will stop iterating and return the
// error immediately.
func (bad *BadBlobs) WalkNamespace(ctx context.Context, namespace []byte, walkFunc func(blobstore.BlobInfo) error) error {
	if err := bad.err.Err(); err != nil {
		return err
	}
	return bad.blobs.WalkNamespace(ctx, namespace, walkFunc)
}

// ListNamespaces returns all namespaces that might be storing data.
func (bad *BadBlobs) ListNamespaces(ctx context.Context) ([][]byte, error) {
	if err := bad.err.Err(); err != nil {
		return make([][]byte, 0), err
	}
	return bad.blobs.ListNamespaces(ctx)
}

// FreeSpace return how much free space left for writing.
func (bad *BadBlobs) FreeSpace(ctx context.Context) (int64, error) {
	if err := bad.err.Err(); err != nil {
		return 0, err
	}
	return bad.blobs.FreeSpace(ctx)
}

// DiskInfo returns information about the disk.
func (bad *BadBlobs) DiskInfo(ctx context.Context) (blobstore.DiskInfo, error) {
	if err := bad.err.Err(); err != nil {
		return blobstore.DiskInfo{}, err
	}
	return bad.blobs.DiskInfo(ctx)
}

// SpaceUsedForBlobs adds up how much is used in all namespaces.
func (bad *BadBlobs) SpaceUsedForBlobs(ctx context.Context) (int64, error) {
	if err := bad.err.Err(); err != nil {
		return 0, err
	}
	return bad.blobs.SpaceUsedForBlobs(ctx)
}

// SpaceUsedForBlobsInNamespace adds up how much is used in the given namespace.
func (bad *BadBlobs) SpaceUsedForBlobsInNamespace(ctx context.Context, namespace []byte) (int64, error) {
	if err := bad.err.Err(); err != nil {
		return 0, err
	}
	return bad.blobs.SpaceUsedForBlobsInNamespace(ctx, namespace)
}

// SpaceUsedForTrash adds up how much is used in all namespaces.
func (bad *BadBlobs) SpaceUsedForTrash(ctx context.Context) (int64, error) {
	if err := bad.err.Err(); err != nil {
		return 0, err
	}
	return bad.blobs.SpaceUsedForTrash(ctx)
}

// CheckWritability tests writability of the storage directory by creating and deleting a file.
func (bad *BadBlobs) CheckWritability(ctx context.Context) error {
	if err := bad.checkErr.Err(); err != nil {
		return err
	}
	return bad.blobs.CheckWritability(ctx)
}

// CreateVerificationFile creates a file to be used for storage directory verification.
func (bad *BadBlobs) CreateVerificationFile(ctx context.Context, id storj.NodeID) error {
	if err := bad.checkErr.Err(); err != nil {
		return err
	}
	return bad.blobs.CreateVerificationFile(ctx, id)
}

// VerifyStorageDir verifies that the storage directory is correct by checking for the existence and validity
// of the verification file.
func (bad *BadBlobs) VerifyStorageDir(ctx context.Context, id storj.NodeID) error {
	if err := bad.checkErr.Err(); err != nil {
		return err
	}
	return bad.blobs.VerifyStorageDir(ctx, id)
}
